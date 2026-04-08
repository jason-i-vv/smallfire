package strategy

import (
	"math"
	"sort"
	"time"

	"github.com/smallfire/starfire/internal/config"
	"github.com/smallfire/starfire/internal/models"
)

// BoxStrategy 箱体突破策略
type BoxStrategy struct {
	config config.BoxStrategyConfig
	deps   Dependency
}

// NewBoxStrategy 创建箱体策略实例
func NewBoxStrategy(cfg config.BoxStrategyConfig, deps Dependency) Strategy {
	return &BoxStrategy{
		config: cfg,
		deps:   deps,
	}
}

func (s *BoxStrategy) Name() string        { return "box_strategy" }
func (s *BoxStrategy) Type() string        { return "box" }
func (s *BoxStrategy) Enabled() bool       { return s.config.Enabled }
func (s *BoxStrategy) Config() interface{} { return s.config }

func (s *BoxStrategy) Analyze(symbolID int, symbolCode, period string, klines []models.Kline) ([]models.Signal, error) {
	if len(klines) < s.config.MinKlines {
		return nil, nil
	}

	var signals []models.Signal
	latestKline := klines[len(klines)-1]

	// 1. 获取已有活跃箱体（同一时间同一周期只允许一个）
	activeBoxes, _ := s.deps.BoxRepo.GetActiveBySymbol(symbolID, period)
	var activeBox *models.Box
	if len(activeBoxes) > 0 {
		activeBox = activeBoxes[0]
	}

	// 2. 如果有活跃箱体：先尝试延续，再检查是否突破
	if activeBox != nil {
		if sig := s.checkBreakout(activeBox, latestKline); sig != nil {
			signals = append(signals, *sig)
		} else {
			// 未突破则尝试延续箱体
			s.tryExtendBox(activeBox, latestKline)
		}
		// 只要有活跃箱体，就不检测新箱体
		return signals, nil
	}

	// 3. 无活跃箱体时，从 K 线中检测新箱体
	boxes := s.detectBoxes(symbolID, period, klines)
	if len(boxes) == 0 {
		return signals, nil
	}

	// 4. 选最新的一个箱体激活
	latestBox := boxes[0]
	for i := 1; i < len(boxes); i++ {
		if boxes[i].EndTime.After(latestBox.EndTime) {
			latestBox = boxes[i]
		}
	}

	// 新箱体入库
	if err := s.deps.BoxRepo.Create(&latestBox); err == nil {
		// 刚创建的箱体也要检查一下是否当前K线就突破了
		if sig := s.checkBreakout(&latestBox, latestKline); sig != nil {
			signals = append(signals, *sig)
		}
	}

	return signals, nil
}

// detectBoxes 检测箱体
// 采用滑动窗口方式，在连续的 Swing 点组合中寻找满足条件的箱体区间
func (s *BoxStrategy) detectBoxes(symbolID int, period string, klines []models.Kline) []models.Box {
	if len(klines) < s.config.MinKlines {
		return nil
	}

	// 1. 检测 Swing 点（波峰波谷）
	swings := s.detectSwingPoints(klines)
	if len(swings) < 4 {
		return nil
	}

	var allBoxes []models.Box

	// 2. 滑动窗口：从每个起始 Swing 点出发，向后扩展，寻找最长的有效箱体
	// 约束：窗口内至少有 2 个高点 + 2 个低点，且高低点价格区间收敛
	for start := 0; start <= len(swings)-4; start++ {
		// 尝试从 start 出发，逐步扩展到更多 Swing 点
		// 每次扩展后检查是否仍满足箱体条件
		for end := start + 3; end < len(swings); end++ {
			window := swings[start : end+1]
			box := s.buildBoxFromSwingRange(symbolID, period, window, klines)
			if box == nil {
				continue
			}
			// 验证箱体有效性（震荡次数、价格收敛、K线在边界内）
			if s.isValidBox(box, window, klines, window[0].Index, window[len(window)-1].Index) {
				allBoxes = append(allBoxes, *box)
				// 找到有效箱体后，直接跳到最长的延伸版本（跳过 end 继续扩展）
			}
		}
	}

	// 3. 对检测到的箱体去重（同一次检测内的重叠箱体保留最大的那个）
	allBoxes = s.deduplicateBoxes(allBoxes)

	// 4. 过滤无效箱体（宽度、K线数等）
	return s.filterValidBoxes(allBoxes, klines)
}

// SwingPoint 波峰波谷
type SwingPoint struct {
	Index int
	Type  SwingType
	Price float64
	Time  time.Time
}

// SwingType 波峰波谷类型
type SwingType int

const (
	SwingHigh SwingType = iota
	SwingLow
)

// detectSwingPoints 检测波峰波谷
func (s *BoxStrategy) detectSwingPoints(klines []models.Kline) []SwingPoint {
	var swings []SwingPoint

	// 使用配置的回溯数，默认 2
	lookback := s.config.SwingLookback
	if lookback < 1 {
		lookback = 2
	}

	// 动态阈值：基于 ATR 计算
	minSwingPercent := s.calculateDynamicThreshold(klines) / 100

	for i := lookback; i < len(klines)-lookback; i++ {
		// 获取前后 lookback 根 K 线的高低点极值
		prevHigh := klines[i-1].HighPrice
		prevLow := klines[i-1].LowPrice
		currHigh := klines[i].HighPrice
		currLow := klines[i].LowPrice
		nextHigh := klines[i+1].HighPrice
		nextLow := klines[i+1].LowPrice

		// 波峰检测：当前 K 线的高点高于前后 lookback 根 K 线的高点
		isHigh := true
		for j := 1; j <= lookback; j++ {
			if i-j >= 0 && klines[i-j].HighPrice > currHigh {
				isHigh = false
				break
			}
			if i+j < len(klines) && klines[i+j].HighPrice > currHigh {
				isHigh = false
				break
			}
		}

		if isHigh && currHigh > prevHigh && currHigh > nextHigh {
			swingPercent := (currHigh - math.Min(prevLow, nextLow)) / currHigh
			if swingPercent >= minSwingPercent {
				swings = append(swings, SwingPoint{
					Index: i,
					Type:  SwingHigh,
					Price: currHigh,
					Time:  klines[i].OpenTime,
				})
			}
		}

		// 波谷检测：当前 K 线的低点低于前后 lookback 根 K 线的低点
		isLow := true
		for j := 1; j <= lookback; j++ {
			if i-j >= 0 && klines[i-j].LowPrice < currLow {
				isLow = false
				break
			}
			if i+j < len(klines) && klines[i+j].LowPrice < currLow {
				isLow = false
				break
			}
		}

		if isLow && currLow < prevLow && currLow < nextLow {
			swingPercent := (math.Max(prevHigh, nextHigh) - currLow) / currLow
			if swingPercent >= minSwingPercent {
				swings = append(swings, SwingPoint{
					Index: i,
					Type:  SwingLow,
					Price: currLow,
					Time:  klines[i].OpenTime,
				})
			}
		}
	}

	return swings
}

// calculateDynamicThreshold 计算动态箱体宽度阈值
// 基于 ATR (Average True Range) 计算，返回百分比值
func (s *BoxStrategy) calculateDynamicThreshold(klines []models.Kline) float64 {
	period := s.config.ATRPeriod
	if period < 5 {
		period = 14 // 默认 ATR 周期
	}

	if len(klines) < period+1 {
		// 数据不足，使用固定阈值
		return s.config.WidthThreshold
	}

	// 计算 ATR：使用最近 period 根 K 线
	lookbackKlines := klines[len(klines)-period-1 : len(klines)-1]

	// 计算 True Range
	var trSum float64
	for i := range lookbackKlines {
		if i == 0 {
			continue
		}
		tr := math.Max(
			lookbackKlines[i].HighPrice-lookbackKlines[i].LowPrice,
			math.Max(
				math.Abs(lookbackKlines[i].HighPrice-lookbackKlines[i-1].ClosePrice),
				math.Abs(lookbackKlines[i].LowPrice-lookbackKlines[i-1].ClosePrice),
			),
		)
		trSum += tr
	}

	atr := trSum / float64(period)

	// 计算 ATR 占当前价格的百分比
	latestClose := klines[len(klines)-1].ClosePrice
	atrPercent := (atr / latestClose) * 100

	// 阈值 = ATR百分比 * 倍数
	threshold := atrPercent * s.config.ATRMultiplier

	// 限制在配置的最大最小范围内
	if threshold < s.config.MinWidthThreshold {
		threshold = s.config.MinWidthThreshold
	}
	if threshold > s.config.MaxWidthThreshold {
		threshold = s.config.MaxWidthThreshold
	}

	return threshold
}

// buildBoxFromSwingRange 从一段连续的Swing区间构建箱体
// 关键修复：箱体边界直接使用窗口内所有K线的实际极值，而非Swing点的价格
func (s *BoxStrategy) buildBoxFromSwingRange(symbolID int, period string, rangeSwings []SwingPoint, klines []models.Kline) *models.Box {
	if len(rangeSwings) < 4 {
		return nil
	}

	// 按时间排序
	sort.Slice(rangeSwings, func(i, j int) bool {
		return rangeSwings[i].Index < rangeSwings[j].Index
	})

	// 获取窗口的K线范围
	firstIdx := rangeSwings[0].Index
	lastIdx := rangeSwings[len(rangeSwings)-1].Index
	boxKlines := klines[firstIdx : lastIdx+1]

	if len(boxKlines) < s.config.MinKlines {
		return nil
	}

	// 关键修复：箱体边界直接使用窗口内所有K线的实际极值
	boxHigh := boxKlines[0].HighPrice
	boxLow := boxKlines[0].LowPrice
	for _, k := range boxKlines {
		if k.HighPrice > boxHigh {
			boxHigh = k.HighPrice
		}
		if k.LowPrice < boxLow {
			boxLow = k.LowPrice
		}
	}

	widthPrice := boxHigh - boxLow
	if widthPrice <= 0 {
		return nil
	}

	return &models.Box{
		SymbolID:     symbolID,
		Period:       period,
		Status:       models.BoxStatusActive,
		HighPrice:    boxHigh,
		LowPrice:     boxLow,
		WidthPrice:   widthPrice,
		WidthPercent: widthPrice / boxLow * 100,
		KlinesCount:  len(boxKlines),
		StartTime:    boxKlines[0].OpenTime,
		EndTime:      time.Time{},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

// filterValidBoxes 过滤无效箱体
func (s *BoxStrategy) filterValidBoxes(boxes []models.Box, klines []models.Kline) []models.Box {
	var validBoxes []models.Box

	// 使用动态阈值
	widthThreshold := s.calculateDynamicThreshold(klines)

	for _, box := range boxes {
		// 宽度过滤：使用动态阈值，且不超过最大宽度
		if box.WidthPercent < widthThreshold {
			continue
		}
		if box.WidthPercent > s.config.MaxWidthThreshold {
			continue
		}

		// 最大K线数限制
		if box.KlinesCount > s.config.MaxKlines {
			continue
		}

		validBoxes = append(validBoxes, box)
	}

	return validBoxes
}

// isValidBox 验证箱体是否满足震荡条件
// 箱体的边界 = 窗口内所有K线的最高价和最低价
// 验证：窗口内任何一根K线超出边界，该"箱体"即无效
func (s *BoxStrategy) isValidBox(box *models.Box, swings []SwingPoint, klines []models.Kline, startIdx, endIdx int) bool {
	if box == nil {
		return false
	}

	// 关键验证：箱体边界由窗口内K线的实际极值决定
	// 如果有任何一根K线超出边界，则箱体定义无效
	boxKlines := klines[startIdx : endIdx+1]
	for _, k := range boxKlines {
		if k.HighPrice > box.HighPrice || k.LowPrice < box.LowPrice {
			return false // 任何一根K线超出边界，箱体即失效
		}
	}

	touchTolerance := box.WidthPrice * 0.05 // 5% 容差

	highTouchCount := 0
	lowTouchCount := 0

	for _, sw := range swings {
		if sw.Type == SwingHigh && sw.Price >= box.HighPrice-touchTolerance {
			highTouchCount++
		}
		if sw.Type == SwingLow && sw.Price <= box.LowPrice+touchTolerance {
			lowTouchCount++
		}
	}

	// 至少需要1个高点和1个低点触及边界
	if highTouchCount < 1 || lowTouchCount < 1 {
		return false
	}

	// 检查单边趋势：计算箱体内高点和低点的最大振幅
	// 如果所有高点或所有低点是单调递增/递减，则是趋势而非箱体
	var highPrices, lowPrices []float64
	for _, sw := range swings {
		if sw.Type == SwingHigh {
			highPrices = append(highPrices, sw.Price)
		} else {
			lowPrices = append(lowPrices, sw.Price)
		}
	}

	// 高点必须有来回，不能单调递减或单调递增
	if len(highPrices) >= 2 {
		if isMonotone(highPrices) {
			return false
		}
	}

	// 低点必须有来回，不能单调递减或单调递增
	if len(lowPrices) >= 2 {
		if isMonotone(lowPrices) {
			return false
		}
	}

	// 新增：检查价格波动性 - 计算窗口内收盘价的来回波动程度
	// volatilityRatio < 1.0 说明价格来回震荡（总位移 < 总路程），是真正的震荡箱体
	// volatilityRatio 接近 1.0 说明价格基本单边移动
	// 注意：震荡行情的 ratio 可能远大于 1.0（多次来回），所以不能以固定值判断
	// 改为：只拒绝真正的单边行情：位移/箱体宽度 >= 0.8（价格从一头走到另一头）
	volatilityRatio := calculateVolatilityRatio(boxKlines)
	if volatilityRatio >= 0.8 { // 单边位移超过箱体宽度 80%，视为趋势行情
		return false
	}

	return true
}

// isMonotone 检查价格序列是否单调（全部递增或全部递减）
func isMonotone(prices []float64) bool {
	if len(prices) < 2 {
		return false
	}
	allUp := true
	allDown := true
	for i := 1; i < len(prices); i++ {
		if prices[i] <= prices[i-1] {
			allUp = false
		}
		if prices[i] >= prices[i-1] {
			allDown = false
		}
	}
	return allUp || allDown
}

// calculateVolatilityRatio 计算价格波动性比率
// 原理：计算窗口内收盘价的净位移（首尾差） / 箱体宽度
// 震荡行情：价格来回波动，净位移远小于箱体宽度，比值小
// 趋势行情：价格单边移动，净位移接近箱体宽度，比值大
// 阈值：>= 0.8 说明净位移超过箱体宽度的80%，视为趋势
func calculateVolatilityRatio(boxKlines []models.Kline) float64 {
	if len(boxKlines) < 2 {
		return 0
	}

	// 净位移：窗口内首根和末根收盘价的差（绝对值）
	netDisplacement := math.Abs(boxKlines[len(boxKlines)-1].ClosePrice - boxKlines[0].ClosePrice)

	// 计算箱体宽度（窗口内所有K线的极值范围）
	boxHigh := boxKlines[0].HighPrice
	boxLow := boxKlines[0].LowPrice
	for _, k := range boxKlines {
		if k.HighPrice > boxHigh {
			boxHigh = k.HighPrice
		}
		if k.LowPrice < boxLow {
			boxLow = k.LowPrice
		}
	}

	boxWidth := boxHigh - boxLow
	if boxWidth <= 0 {
		return 0
	}

	return netDisplacement / boxWidth
}

// deduplicateBoxes 对同一次检测内的箱体去重
// 保留价格区间重叠度高的箱体中 KlinesCount 最多的那个（最完整的箱体）
func (s *BoxStrategy) deduplicateBoxes(boxes []models.Box) []models.Box {
	if len(boxes) <= 1 {
		return boxes
	}

	kept := make([]bool, len(boxes))
	for i := range kept {
		kept[i] = true
	}

	for i := 0; i < len(boxes); i++ {
		if !kept[i] {
			continue
		}
		for j := i + 1; j < len(boxes); j++ {
			if !kept[j] {
				continue
			}
			// 使用包含比（小箱体被大箱体包含时重叠度高）
			// 优先保留较窄的箱体（KlinesCount 较少）——窄箱体代表更紧凑的震荡区间，突破时机更有价值
			overlap := calculateContainmentOverlap(boxes[i].LowPrice, boxes[i].HighPrice, boxes[j].LowPrice, boxes[j].HighPrice)
			if overlap > 0.7 {
				// 保留 KlinesCount 较少的那个（更窄的箱体）
				if boxes[i].KlinesCount <= boxes[j].KlinesCount {
					kept[j] = false
				} else {
					kept[i] = false
					break
				}
			}
		}
	}

	var result []models.Box
	for i, box := range boxes {
		if kept[i] {
			result = append(result, box)
		}
	}
	return result
}

// isKnownBox 检查是否是已知箱体
func (s *BoxStrategy) isKnownBox(box models.Box, activeBoxes []*models.Box) bool {
	for _, ab := range activeBoxes {
		// 相似度判断：价格范围重叠度 > 80% 且时间范围有重叠
		if s.isOverlappingBox(&box, ab) {
			return true
		}
	}
	return false
}

// isOverlappingBox 判断两个箱体是否重叠
func (s *BoxStrategy) isOverlappingBox(a, b *models.Box) bool {
	// 首先检查周期是否相同，不同周期的箱体不认为是重叠的
	if a.Period != b.Period {
		return false
	}

	// 价格范围重叠度
	priceOverlap := calculateOverlap(a.LowPrice, a.HighPrice, b.LowPrice, b.HighPrice)

	// 时间范围重叠判断
	var timeOverlap bool
	if a.EndTime.IsZero() || b.EndTime.IsZero() {
		// 如果任一箱体是活跃状态（没有结束时间），判断时间是否有重叠
		if a.EndTime.IsZero() && b.EndTime.IsZero() {
			// 两个都是活跃箱体 - 判断它们的起始时间是否相近
			// 计算两个箱体起始时间的差距
			timeDiff := a.StartTime.Sub(b.StartTime).Abs()
			// 如果起始时间差小于等于箱体包含K线数量的一半时间，则认为是重叠
			// 例如：如果箱体包含20根K线，则时间差 <= 10 * 周期时长认为是重叠
			maxKlines := a.KlinesCount
			if b.KlinesCount > maxKlines {
				maxKlines = b.KlinesCount
			}
			// 获取周期对应的时长
			periodDuration := getPeriodDurationForComparison(a.Period)
			// 允许的时间差 = maxKlines * 周期时长 / 2
			allowedTimeDiff := periodDuration * time.Duration(maxKlines) / 2
			// 如果时间差小于允许的时间差，则认为是重叠
			timeOverlap = timeDiff <= allowedTimeDiff
		} else if a.EndTime.IsZero() {
			// a是活跃箱体，b有结束时间
			// 判断a的起始时间是否在b的时间范围内，或者之后但时间相近
			if a.StartTime.Before(b.StartTime) {
				// a在b之前，不算重叠
				timeOverlap = false
			} else if a.StartTime.After(b.EndTime) {
				// a在b结束之后，判断时间差
				timeDiff := a.StartTime.Sub(b.EndTime)
				periodDuration := getPeriodDurationForComparison(a.Period)
				// 允许的时间差 = 箱体K线数量 * 周期时长
				allowedTimeDiff := periodDuration * time.Duration(a.KlinesCount)
				timeOverlap = timeDiff <= allowedTimeDiff
			} else {
				// a的起始时间在b的时间范围内，认为是重叠
				timeOverlap = true
			}
		} else {
			// b是活跃箱体，a有结束时间 - 与上面对称
			if b.StartTime.Before(a.StartTime) {
				timeOverlap = false
			} else if b.StartTime.After(a.EndTime) {
				timeDiff := b.StartTime.Sub(a.EndTime)
				periodDuration := getPeriodDurationForComparison(a.Period)
				allowedTimeDiff := periodDuration * time.Duration(b.KlinesCount)
				timeOverlap = timeDiff <= allowedTimeDiff
			} else {
				timeOverlap = true
			}
		}
	} else {
		// 两个都是已关闭的箱体，判断正常的时间重叠
		timeOverlap = a.StartTime.Before(b.EndTime) && b.StartTime.Before(a.EndTime)
	}

	// 价格重叠度 > 80% 且时间有重叠
	return priceOverlap > 0.8 && timeOverlap
}

// getPeriodDurationForComparison 获取用于比较的周期时长
func getPeriodDurationForComparison(period string) time.Duration {
	switch period {
	case "1m":
		return time.Minute
	case "5m":
		return 5 * time.Minute
	case "15m":
		return 15 * time.Minute
	case "30m":
		return 30 * time.Minute
	case "1h":
		return time.Hour
	case "4h":
		return 4 * time.Hour
	case "1d":
		return 24 * time.Hour
	default:
		return 15 * time.Minute // 默认15分钟
	}
}

// calculateOverlap 计算价格范围重叠度（基于并集，用于判断两个箱体是否高度重叠）
func calculateOverlap(aLow, aHigh, bLow, bHigh float64) float64 {
	overlapLow := math.Max(aLow, bLow)
	overlapHigh := math.Min(aHigh, bHigh)
	if overlapLow >= overlapHigh {
		return 0
	}
	overlapWidth := overlapHigh - overlapLow
	totalWidth := math.Max(aHigh, bHigh) - math.Min(aLow, bLow)
	if totalWidth == 0 {
		return 0
	}
	return overlapWidth / totalWidth
}

// calculateContainmentOverlap 计算包含重叠度
// 当一个箱体完全被另一个箱体包含时，返回值接近1.0
// 取重叠区间占【较小箱体宽度】的比例，避免小箱体被大箱体淹没时漏判
func calculateContainmentOverlap(aLow, aHigh, bLow, bHigh float64) float64 {
	overlapLow := math.Max(aLow, bLow)
	overlapHigh := math.Min(aHigh, bHigh)
	if overlapLow >= overlapHigh {
		return 0
	}
	overlapWidth := overlapHigh - overlapLow
	aWidth := aHigh - aLow
	bWidth := bHigh - bLow
	minWidth := math.Min(aWidth, bWidth)
	if minWidth == 0 {
		return 0
	}
	return overlapWidth / minWidth
}

// tryExtendBox 尝试延续活跃箱体
// 如果最新K线的实体在箱体范围内，则扩展箱体的 KlinesCount
// 如果K线实体超出箱体范围但未达到突破阈值，则箱体失效
func (s *BoxStrategy) tryExtendBox(box *models.Box, latestKline models.Kline) {
	buffer := box.WidthPrice * s.config.BreakoutBuffer

	// 使用 K 线的高低价判断：实体在边界内才算在箱体中
	// 同时考虑影线：影线可以轻微超出，但不应大幅突破
	highInRange := latestKline.HighPrice <= box.HighPrice+buffer
	lowInRange := latestKline.LowPrice >= box.LowPrice-buffer

	// K 线实体（开盘-收盘）在边界内，且高低点未大幅突破
	if highInRange && lowInRange {
		box.KlinesCount++
		box.EndTime = latestKline.OpenTime
		box.UpdatedAt = time.Now()
		s.deps.BoxRepo.Update(box)
	}
}

// checkBreakout 检查是否有效突破
func (s *BoxStrategy) checkBreakout(box *models.Box, latestKline models.Kline) *models.Signal {
	latestPrice := latestKline.ClosePrice
	boxHigh := box.HighPrice
	boxLow := box.LowPrice
	boxWidth := boxHigh - boxLow

	buffer := boxWidth * s.config.BreakoutBuffer

	// 向上突破
	if latestPrice > boxHigh+buffer {
		// 更新箱体状态
		box.Status = models.BoxStatusClosed
		box.EndTime = latestKline.OpenTime
		box.BreakoutPrice = &latestPrice
		dir := models.BreakoutDirectionUp
		box.BreakoutDirection = &dir
		s.deps.BoxRepo.Update(box)

		// 生成信号
		return s.createBreakoutSignal(*box, latestKline, "long", latestPrice)
	}

	// 向下突破
	if latestPrice < boxLow-buffer {
		box.Status = models.BoxStatusClosed
		box.EndTime = latestKline.OpenTime
		box.BreakoutPrice = &latestPrice
		dir := models.BreakoutDirectionDown
		box.BreakoutDirection = &dir
		s.deps.BoxRepo.Update(box)

		return s.createBreakoutSignal(*box, latestKline, "short", latestPrice)
	}

	return nil
}

// createBreakoutSignal 创建突破信号
func (s *BoxStrategy) createBreakoutSignal(box models.Box, kline models.Kline, direction string, price float64) *models.Signal {
	// 计算信号强度
	strength := s.calculateStrength(box)

	// 计算止盈止损
	stopLoss := s.calculateStopLoss(box, direction)
	target := s.calculateTarget(box, direction)

	signalType := models.SignalTypeBoxBreakout
	if direction == "short" {
		signalType = models.SignalTypeBoxBreakdown
	}

	expireTime := time.Now().Add(24 * time.Hour)

	return &models.Signal{
		SymbolID:         kline.SymbolID,
		SignalType:       signalType,
		SourceType:       models.SourceTypeBox,
		Direction:        direction,
		Strength:         strength,
		Price:            price,
		TargetPrice:      &target,
		StopLossPrice:    &stopLoss,
		Period:           kline.Period,
		SignalData:       &models.JSONB{},
		Status:           models.SignalStatusPending,
		ExpiredAt:        &expireTime,
		NotificationSent: false,
		CreatedAt:        time.Now(),
		KlineTime:        ptrTime(kline.OpenTime),
	}
}

// calculateStrength 计算信号强度
func (s *BoxStrategy) calculateStrength(box models.Box) int {
	if box.WidthPercent > 5 && box.KlinesCount >= 20 {
		return 3 // 强
	} else if box.WidthPercent > 2 && box.KlinesCount >= 10 {
		return 2 // 中
	}
	return 1 // 弱
}

// calculateStopLoss 计算止损价格
func (s *BoxStrategy) calculateStopLoss(box models.Box, direction string) float64 {
	buffer := (box.HighPrice - box.LowPrice) * 0.005 // 0.5%缓冲
	if direction == "long" {
		return box.LowPrice - buffer
	}
	return box.HighPrice + buffer
}

// calculateTarget 计算目标价格
func (s *BoxStrategy) calculateTarget(box models.Box, direction string) float64 {
	width := box.HighPrice - box.LowPrice
	if direction == "long" {
		return box.HighPrice + width*1.5
	}
	return box.LowPrice - width*1.5
}
