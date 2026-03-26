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

	// 1. 检测箱体
	boxes := s.detectBoxes(symbolID, klines)

	// 2. 检查已有箱体状态
	activeBoxes, _ := s.deps.BoxRepo.GetActiveBySymbol(symbolID, period)

	for _, box := range boxes {
		if s.isKnownBox(box, activeBoxes) {
			// 检查是否突破
			if sig := s.checkBreakout(&box, klines[len(klines)-1]); sig != nil {
				signals = append(signals, *sig)
			}
		} else {
			// 新箱体，先创建再检查突破
			if err := s.deps.BoxRepo.Create(&box); err == nil {
				if sig := s.checkBreakout(&box, klines[len(klines)-1]); sig != nil {
					signals = append(signals, *sig)
				}
			}
		}
	}

	return signals, nil
}

// detectBoxes 检测箱体
func (s *BoxStrategy) detectBoxes(symbolID int, klines []models.Kline) []models.Box {
	if len(klines) < 5 {
		return nil
	}

	// 1. 检测Swing点（波峰波谷）
	swings := s.detectSwingPoints(klines)

	// 2. 从Swing点构建箱体
	boxes := s.buildBoxesFromSwings(symbolID, swings, klines)

	// 3. 过滤无效箱体
	return s.filterValidBoxes(boxes, klines)
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
	minSwingPercent := s.config.WidthThreshold / 100

	for i := 2; i < len(klines)-2; i++ {
		prevHigh := klines[i-1].HighPrice
		currHigh := klines[i].HighPrice
		nextHigh := klines[i+1].HighPrice

		prevLow := klines[i-1].LowPrice
		currLow := klines[i].LowPrice
		nextLow := klines[i+1].LowPrice

		// 波峰检测
		if currHigh > prevHigh && currHigh > nextHigh {
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

		// 波谷检测
		if currLow < prevLow && currLow < nextLow {
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

// buildBoxesFromSwings 从波峰波谷构建箱体
func (s *BoxStrategy) buildBoxesFromSwings(symbolID int, swings []SwingPoint, klines []models.Kline) []models.Box {
	var boxes []models.Box

	for i := 0; i < len(swings)-1; i++ {
		startSwing := swings[i]
		endSwing := swings[i+1]

		// 只处理不同类型的连续Swing点
		if startSwing.Type == endSwing.Type {
			continue
		}

		// 提取箱体的K线
		startIdx := startSwing.Index
		endIdx := endSwing.Index
		if startIdx > endIdx {
			startIdx, endIdx = endIdx, startIdx
		}
		boxKlines := klines[startIdx : endIdx+1]
		if len(boxKlines) < s.config.MinKlines {
			continue
		}

		// 计算箱体边界
		var highs, lows []float64
		for _, k := range boxKlines {
			highs = append(highs, k.HighPrice)
			lows = append(lows, k.LowPrice)
		}

		sort.Float64s(highs)
		sort.Float64s(lows)

		box := models.Box{
			SymbolID:     symbolID,
			Status:       models.BoxStatusActive,
			HighPrice:    highs[len(highs)-1],
			LowPrice:     lows[0],
			WidthPrice:   highs[len(highs)-1] - lows[0],
			WidthPercent: (highs[len(highs)-1] - lows[0]) / lows[0] * 100,
			KlinesCount:  len(boxKlines),
			StartTime:    boxKlines[0].OpenTime,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		boxes = append(boxes, box)
	}

	return boxes
}

// filterValidBoxes 过滤无效箱体
func (s *BoxStrategy) filterValidBoxes(boxes []models.Box, klines []models.Kline) []models.Box {
	var validBoxes []models.Box

	for _, box := range boxes {
		// 宽度过滤
		if box.WidthPercent < s.config.WidthThreshold {
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
	// 价格范围重叠度
	priceOverlap := calculateOverlap(a.LowPrice, a.HighPrice, b.LowPrice, b.HighPrice)

	// 时间范围重叠 - 如果任一箱体没有结束时间（active 状态），只判断价格重叠
	if a.EndTime == nil || b.EndTime == nil {
		return priceOverlap > 0.8
	}

	timeOverlap := a.StartTime.Before(*b.EndTime) && b.StartTime.Before(*a.EndTime)
	return priceOverlap > 0.8 && timeOverlap
}

// calculateOverlap 计算价格范围重叠度
func calculateOverlap(aLow, aHigh, bLow, bHigh float64) float64 {
	// 计算重叠区域
	overlapLow := math.Max(aLow, bLow)
	overlapHigh := math.Min(aHigh, bHigh)

	if overlapLow >= overlapHigh {
		return 0
	}

	overlapWidth := overlapHigh - overlapLow
	totalWidth := math.Max(aHigh, bHigh) - math.Min(aLow, bLow)

	return overlapWidth / totalWidth
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
		box.EndTime = &latestKline.OpenTime
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
		box.EndTime = &latestKline.OpenTime
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
