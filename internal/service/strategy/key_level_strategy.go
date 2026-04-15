package strategy

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/smallfire/starfire/internal/config"
	"github.com/smallfire/starfire/internal/models"
)

// KeyLevelStrategy 阻力支撑策略
// 算法识别关键价位（swing point + volume profile + 整数关口），监控突破生成信号
type KeyLevelStrategy struct {
	config config.KeyLevelStrategyConfig
	deps   Dependency
}

// NewKeyLevelStrategy 创建阻力支撑策略实例
func NewKeyLevelStrategy(cfg config.KeyLevelStrategyConfig, deps Dependency) Strategy {
	return &KeyLevelStrategy{
		config: cfg,
		deps:   deps,
	}
}

func (s *KeyLevelStrategy) Name() string        { return "key_level_strategy" }
func (s *KeyLevelStrategy) Type() string        { return "key_level" }
func (s *KeyLevelStrategy) Enabled() bool       { return s.config.Enabled }
func (s *KeyLevelStrategy) Config() interface{} { return s.config }

// candidateLevel 候选关键价位
type candidateLevel struct {
	price           float64
	levelType       string // "resistance" | "support"
	touchCount      int
	isVolumeCluster bool
	isRoundNumber   bool
}

func (s *KeyLevelStrategy) Analyze(symbolID int, symbolCode, period string, klines []models.Kline) ([]models.Signal, error) {
	if len(klines) < 30 {
		return nil, nil
	}

	latestKline := klines[len(klines)-1]
	latestPrice := latestKline.ClosePrice
	latestIdx := len(klines) - 1

	// 1. 算法识别关键价位
	resistances, supports := s.detectKeyLevels(klines)

	// 2. 写入 V2 表（Upsert 覆盖）
	if s.deps.LevelV2Repo != nil && (len(resistances) > 0 || len(supports) > 0) {
		if err := s.deps.LevelV2Repo.Upsert(symbolID, period, resistances, supports); err != nil {
			// 写入失败不影响突破检测
			_ = err
		}
	}

	// 3. 突破检测 + 信号生成
	if s.deps.LevelV2Repo == nil {
		return nil, nil
	}
	levelPeriod := period
	if levelPeriod != "1h" {
		levelPeriod = "1h"
	}
	v2Levels, err := s.deps.LevelV2Repo.GetBySymbolPeriod(symbolID, levelPeriod)
	if err != nil || v2Levels == nil {
		return nil, nil
	}

	threshold := s.config.LevelDistance / 100.0
	volumeConfirmed := s.checkVolumeConfirm(klines, latestIdx)

	var signals []models.Signal

	// 阻力位突破（向上）
	var closestBrokenResistance *models.KeyLevelEntry
	for i := range v2Levels.Resistances {
		level := &v2Levels.Resistances[i]
		breakoutPrice := level.Price * (1 + threshold)
		if latestPrice > breakoutPrice {
			if !volumeConfirmed {
				continue
			}
			if closestBrokenResistance == nil || level.Price < closestBrokenResistance.Price {
				closestBrokenResistance = level
			}
		}
	}
	if closestBrokenResistance != nil {
		signals = append(signals, s.createBreakSignal(symbolID, *closestBrokenResistance, models.LevelTypeResistance, latestKline, "long"))
	}

	// 支撑位突破（向下）
	var closestBrokenSupport *models.KeyLevelEntry
	for i := range v2Levels.Supports {
		level := &v2Levels.Supports[i]
		breakoutPrice := level.Price * (1 - threshold)
		if latestPrice < breakoutPrice {
			if !volumeConfirmed {
				continue
			}
			if closestBrokenSupport == nil || level.Price > closestBrokenSupport.Price {
				closestBrokenSupport = level
			}
		}
	}
	if closestBrokenSupport != nil {
		signals = append(signals, s.createBreakSignal(symbolID, *closestBrokenSupport, models.LevelTypeSupport, latestKline, "short"))
	}

	// 同一K线同时产生做多和做空信号时，选择突破距离更大的一方
	if len(signals) == 2 {
		var longSig, shortSig *models.Signal
		for i := range signals {
			if signals[i].Direction == "long" {
				longSig = &signals[i]
			} else {
				shortSig = &signals[i]
			}
		}
		if longSig != nil && shortSig != nil {
			longDist := (*longSig.SignalData)["level_distance"].(float64)
			shortDist := (*shortSig.SignalData)["level_distance"].(float64)
			if longDist > 0 && shortDist > 0 && math.Abs(longDist-shortDist)/math.Max(longDist, shortDist) < 0.3 {
				longStr := strengthToInt((*longSig.SignalData)["level_strength"].(string))
				shortStr := strengthToInt((*shortSig.SignalData)["level_strength"].(string))
				if longStr >= shortStr {
					signals = []models.Signal{*longSig}
				} else {
					signals = []models.Signal{*shortSig}
				}
			} else if longDist >= shortDist {
				signals = []models.Signal{*longSig}
			} else {
				signals = []models.Signal{*shortSig}
			}
		}
	}

	return signals, nil
}

// ==================== 算法识别 ====================

// detectKeyLevels 算法识别关键价位，返回阻力和支撑列表
func (s *KeyLevelStrategy) detectKeyLevels(klines []models.Kline) ([]models.KeyLevelEntry, []models.KeyLevelEntry) {
	latestPrice := klines[len(klines)-1].ClosePrice

	// 计算 ATR 用于动态阈值
	atr, atrPercent := s.calcATR(klines, 14)

	var candidates []candidateLevel

	// 1. Swing Point 检测（带 ATR prominence 过滤）
	candidates = append(candidates, s.detectSwingPoints(klines, atr)...)

	// 2. Volume Profile
	candidates = append(candidates, s.detectVolumeClusters(klines)...)

	// 3. 整数关口
	candidates = append(candidates, s.detectRoundNumbers(latestPrice)...)

	// 4. 重新分类方向：以当前价格为基准，高于当前价=阻力，低于=支撑
	for i := range candidates {
		if candidates[i].price > latestPrice {
			candidates[i].levelType = models.LevelTypeResistance
		} else {
			candidates[i].levelType = models.LevelTypeSupport
		}
	}

	// 5. 距离过滤：动态范围 ±max(3%, ATR%×3)，高波动币自动扩大
	maxDist := 3.0
	if atrPercent > 0 {
		atrDist := atrPercent * 3.0
		if atrDist > maxDist {
			maxDist = atrDist
		}
		if maxDist > 8.0 {
			maxDist = 8.0
		}
	}
	var nearby []candidateLevel
	for _, c := range candidates {
		dist := math.Abs(c.price-latestPrice) / latestPrice * 100
		if dist <= maxDist {
			nearby = append(nearby, c)
		}
	}
	if len(nearby) == 0 {
		// 放宽到 maxDist×1.5
		for _, c := range candidates {
			dist := math.Abs(c.price-latestPrice) / latestPrice * 100
			if dist <= maxDist*1.5 {
				nearby = append(nearby, c)
			}
		}
	}

	// 6. 计算 touch count（仅统计近 60 根 K 线）
	for i := range nearby {
		nearby[i].touchCount = s.countRecentTouches(klines, nearby[i].price, 60)
	}

	// 7. 过滤：至少 2 次触及 或 是 volume cluster（密集成交区不需要触及）
	var filtered []candidateLevel
	for _, c := range nearby {
		if c.touchCount >= 2 || c.isVolumeCluster {
			filtered = append(filtered, c)
		}
	}

	// 8. 合并聚类 + 分类（使用 ATR 动态容忍度）
	mergeTolerance := 0.003 // 默认 0.3%
	if atrPercent > 0 {
		// ATR% × 0.5，至少 0.2%，最多 1.0%
		mergeTolerance = atrPercent * 0.01 // 同一个 ATR 范围内的价位视为同一区域
		if mergeTolerance < 0.003 {
			mergeTolerance = 0.003
		}
		if mergeTolerance > 0.02 {
			mergeTolerance = 0.02
		}
	}
	resistances := s.mergeAndClassify(filtered, models.LevelTypeResistance, latestPrice, mergeTolerance)
	supports := s.mergeAndClassify(filtered, models.LevelTypeSupport, latestPrice, mergeTolerance)

	return resistances, supports
}

// detectSwingPoints 检测 swing high/low 作为候选价位（带 ATR prominence 过滤）
func (s *KeyLevelStrategy) detectSwingPoints(klines []models.Kline, atr float64) []candidateLevel {
	var candidates []candidateLevel
	lookback := 5

	// ATR prominence 门槛：swing 的幅度至少要 > ATR × 0.5 才有意义
	minSwingMove := atr * 0.5
	if minSwingMove <= 0 {
		// ATR 无效时回退到价格的 0.3%
		if len(klines) > 0 {
			minSwingMove = klines[len(klines)-1].ClosePrice * 0.003
		}
	}

	startIdx := 0
	lk := s.config.LookbackKlines
	if lk <= 0 {
		lk = 200
	}
	if len(klines) > lk {
		startIdx = len(klines) - lk
	}

	// 先找所有 swing point
	type swingInfo struct {
		idx       int
		price     float64
		isHigh    bool
		amplitude float64 // swing 相对周围的高低点幅度
	}
	var swings []swingInfo

	for i := startIdx + lookback; i < len(klines)-lookback; i++ {
		currHigh := klines[i].HighPrice
		currLow := klines[i].LowPrice

		// 波峰检测
		isHigh := true
		surroundLow := currHigh
		for j := 1; j <= lookback; j++ {
			if i-j >= 0 {
				if klines[i-j].HighPrice > currHigh {
					isHigh = false
					break
				}
				if klines[i-j].LowPrice < surroundLow {
					surroundLow = klines[i-j].LowPrice
				}
			}
			if i+j < len(klines) {
				if klines[i+j].HighPrice > currHigh {
					isHigh = false
					break
				}
				if klines[i+j].LowPrice < surroundLow {
					surroundLow = klines[i+j].LowPrice
				}
			}
		}
		midPrice := (currHigh + currLow) / 2
		prevHigh := klines[i-1].HighPrice
		nextHigh := klines[i+1].HighPrice
		if isHigh && currHigh > prevHigh && currHigh > nextHigh && klines[i].ClosePrice > midPrice {
			amplitude := currHigh - surroundLow
			swings = append(swings, swingInfo{idx: i, price: currHigh, isHigh: true, amplitude: amplitude})
		}

		// 波谷检测
		isLow := true
		surroundHigh := currLow
		for j := 1; j <= lookback; j++ {
			if i-j >= 0 {
				if klines[i-j].LowPrice < currLow {
					isLow = false
					break
				}
				if klines[i-j].HighPrice > surroundHigh {
					surroundHigh = klines[i-j].HighPrice
				}
			}
			if i+j < len(klines) {
				if klines[i+j].LowPrice < currLow {
					isLow = false
					break
				}
				if klines[i+j].HighPrice > surroundHigh {
					surroundHigh = klines[i+j].HighPrice
				}
			}
		}
		prevLow := klines[i-1].LowPrice
		nextLow := klines[i+1].LowPrice
		if isLow && currLow < prevLow && currLow < nextLow && klines[i].ClosePrice < midPrice {
			amplitude := surroundHigh - currLow
			swings = append(swings, swingInfo{idx: i, price: currLow, isHigh: false, amplitude: amplitude})
		}
	}

	// ATR prominence 过滤：只保留幅度 >= minSwingMove 的 swing point
	for _, sw := range swings {
		if sw.amplitude >= minSwingMove {
			levelType := models.LevelTypeResistance
			if !sw.isHigh {
				levelType = models.LevelTypeSupport
			}
			candidates = append(candidates, candidateLevel{
				price:     sw.price,
				levelType: levelType,
			})
		}
	}

	return candidates
}

// detectVolumeClusters 统计成交量价格分布，找出密集成交区
func (s *KeyLevelStrategy) detectVolumeClusters(klines []models.Kline) []candidateLevel {
	if len(klines) < 20 {
		return nil
	}

	// 找价格范围
	var highPrice, lowPrice float64
	for i, k := range klines {
		if i == 0 || k.HighPrice > highPrice {
			highPrice = k.HighPrice
		}
		if i == 0 || k.LowPrice < lowPrice {
			lowPrice = k.LowPrice
		}
	}
	if highPrice <= lowPrice {
		return nil
	}

	// 分 50 个区间
	buckets := 50
	rangeSize := (highPrice - lowPrice) / float64(buckets)
	volumes := make([]float64, buckets)

	for _, k := range klines {
		// K线的成交量按价格区间分配
		kRange := k.HighPrice - k.LowPrice
		if kRange <= 0 {
			continue
		}
		for b := 0; b < buckets; b++ {
			bucketLow := lowPrice + float64(b)*rangeSize
			bucketHigh := bucketLow + rangeSize
			// K线与区间重叠部分
			overlapLow := math.Max(k.LowPrice, bucketLow)
			overlapHigh := math.Min(k.HighPrice, bucketHigh)
			if overlapHigh > overlapLow {
				ratio := (overlapHigh - overlapLow) / kRange
				volumes[b] += k.Volume * ratio
			}
		}
	}

	// 计算阈值：top 20% 成交量
	sortedVol := make([]float64, buckets)
	copy(sortedVol, volumes)
	sort.Float64s(sortedVol)
	thresholdIdx := int(float64(buckets) * 0.8)
	if thresholdIdx >= buckets {
		thresholdIdx = buckets - 1
	}
	volThreshold := sortedVol[thresholdIdx]

	var candidates []candidateLevel
	for b := 0; b < buckets; b++ {
		if volumes[b] >= volThreshold && volumes[b] > 0 {
			price := lowPrice + (float64(b)+0.5)*rangeSize
			// 判断是阻力还是支撑（相对当前价格）
			latestPrice := klines[len(klines)-1].ClosePrice
			levelType := models.LevelTypeResistance
			if price < latestPrice {
				levelType = models.LevelTypeSupport
			}
			candidates = append(candidates, candidateLevel{
				price:           price,
				levelType:       levelType,
				isVolumeCluster: true,
			})
		}
	}

	return candidates
}

// detectRoundNumbers 根据价格量级识别整数关口
func (s *KeyLevelStrategy) detectRoundNumbers(currentPrice float64) []candidateLevel {
	if currentPrice <= 0 {
		return nil
	}

	// 根据价格量级确定整数关口间距
	var steps []float64
	switch {
	case currentPrice < 1:
		steps = []float64{0.01, 0.05, 0.1}
	case currentPrice < 10:
		steps = []float64{0.1, 0.5, 1}
	case currentPrice < 100:
		steps = []float64{1, 5, 10}
	case currentPrice < 10000:
		steps = []float64{10, 50, 100, 500, 1000}
	default:
		steps = []float64{100, 500, 1000, 5000, 10000}
	}

	// 当前价格 ±5% 范围
	low := currentPrice * 0.95
	high := currentPrice * 1.05

	seen := make(map[float64]bool)
	var candidates []candidateLevel

	minStep := currentPrice * 0.005 // 步长至少 0.5% 价格才有心理意义
	for _, step := range steps {
		if step < minStep {
			continue
		}
		// 找范围内所有该间距的整数关口
		start := math.Floor(low/step) * step
		for p := start; p <= high; p += step {
			if p <= 0 || seen[p] {
				continue
			}
			seen[p] = true
			levelType := models.LevelTypeResistance
			if p < currentPrice {
				levelType = models.LevelTypeSupport
			}
			candidates = append(candidates, candidateLevel{
				price:         p,
				levelType:     levelType,
				isRoundNumber: true,
			})
		}
	}

	return candidates
}

// countRecentTouches 统计价位在最近 recentN 根 K 线中被触及的次数
func (s *KeyLevelStrategy) countRecentTouches(klines []models.Kline, price float64, recentN int) int {
	if price <= 0 {
		return 0
	}
	tolerance := price * 0.003 // 0.3% 容差
	count := 0
	start := 0
	if len(klines) > recentN {
		start = len(klines) - recentN
	}
	for _, k := range klines[start:] {
		// 阻力触及：最高价接近但收盘未站上
		if k.HighPrice >= price-tolerance && k.ClosePrice <= price {
			count++
			continue
		}
		// 支撑触及：最低价接近但收盘未跌破
		if k.LowPrice <= price+tolerance && k.ClosePrice >= price {
			count++
		}
	}
	return count
}

// mergeAndClassify 合并聚类 + 分类，返回最多 3 个价位（strong/mid/weak）
func (s *KeyLevelStrategy) mergeAndClassify(candidates []candidateLevel, levelType string, latestPrice float64, tolerance float64) []models.KeyLevelEntry {
	// 过滤出该类型
	var filtered []candidateLevel
	for _, c := range candidates {
		if c.levelType != levelType {
			continue
		}
		filtered = append(filtered, c)
	}
	if len(filtered) == 0 {
		return nil
	}

	// 按价格排序
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].price < filtered[j].price
	})

	// 聚类：相近价位合并（tolerance 由 ATR 动态计算）
	var merged []candidateLevel
	for _, c := range filtered {
		found := false
		for i := range merged {
			if math.Abs(merged[i].price-c.price)/c.price < tolerance {
				// 合并：取更精确的价格，累加信号
				if c.touchCount > merged[i].touchCount {
					merged[i].price = c.price
				}
				merged[i].touchCount += c.touchCount
				if c.isVolumeCluster {
					merged[i].isVolumeCluster = true
				}
				if c.isRoundNumber {
					merged[i].isRoundNumber = true
				}
				found = true
				break
			}
		}
		if !found {
			merged = append(merged, c)
		}
	}

	// 评分 + 分类
	type scored struct {
		entry    models.KeyLevelEntry
		score    int
		distance float64
	}
	var scoredList []scored
	for _, m := range merged {
		score := 0
		var reasons []string

		// touch count 评分（高频触及加分）
		switch {
		case m.touchCount >= 10:
			score += 4
			reasons = append(reasons, fmt.Sprintf("%d次触及", m.touchCount))
		case m.touchCount >= 4:
			score += 3
			reasons = append(reasons, fmt.Sprintf("%d次触及", m.touchCount))
		case m.touchCount >= 3:
			score += 2
			reasons = append(reasons, fmt.Sprintf("%d次触及", m.touchCount))
		case m.touchCount >= 2:
			score += 1
			reasons = append(reasons, fmt.Sprintf("%d次触及", m.touchCount))
		}

		// volume cluster 评分
		if m.isVolumeCluster {
			score += 2
			reasons = append(reasons, "密集成交")
		}

		// round number 评分
		if m.isRoundNumber {
			score += 1
			reasons = append(reasons, "整数关口")
		}

		// 分类
		strength := models.LevelStrengthWeak
		switch {
		case score >= 5:
			strength = models.LevelStrengthStrong
		case score >= 3:
			strength = models.LevelStrengthMid
		}

		// 拼接 reason
		reason := "算法识别"
		if len(reasons) > 0 {
			reason = reasons[0]
			for i := 1; i < len(reasons); i++ {
				reason += "+" + reasons[i]
			}
		}

		distance := math.Abs(m.price-latestPrice) / latestPrice * 100
		scoredList = append(scoredList, scored{
			entry: models.KeyLevelEntry{
				Price:    m.price,
				Strength: strength,
				Reason:   reason,
			},
			score:    score,
			distance: distance,
		})
	}

	// 按分数降序排序，分数相同按距离升序
	sort.Slice(scoredList, func(i, j int) bool {
		if scoredList[i].score != scoredList[j].score {
			return scoredList[i].score > scoredList[j].score
		}
		return scoredList[i].distance < scoredList[j].distance
	})

	// 按分数排序取前 3 个（纯按质量，不强制填充弱位）
	maxPerDirection := 3
	result := make([]models.KeyLevelEntry, 0, maxPerDirection)
	for _, s := range scoredList {
		if len(result) >= maxPerDirection {
			break
		}
		result = append(result, s.entry)
	}

	return result
}

// ==================== 信号生成 ====================

// createBreakSignal 创建突破信号
func (s *KeyLevelStrategy) createBreakSignal(symbolID int, level models.KeyLevelEntry, levelType string, kline models.Kline, direction string) models.Signal {
	price := kline.ClosePrice
	distance := math.Abs(price-level.Price) / level.Price * 100

	levelLabel := "阻力位"
	actionLabel := "突破"
	if levelType == models.LevelTypeSupport {
		levelLabel = "支撑位"
		actionLabel = "跌破"
	}

	signalType := models.SignalTypeResistanceBreak
	if levelType == models.LevelTypeSupport {
		signalType = models.SignalTypeSupportBreak
	}

	strength := strengthToInt(level.Strength)

	desc := fmt.Sprintf("ALGO:%s %s%s，%s，距%s%.2f%%",
		level.Reason, actionLabel, levelLabel, level.Strength, actionLabel, distance)

	return models.Signal{
		SymbolID:       symbolID,
		SignalType:     signalType,
		SourceType:     models.SourceTypeKeyLevel,
		Direction:      direction,
		Strength:       strength,
		Price:          price,
		StopLossPrice:  &level.Price,
		Period:         kline.Period,
		SignalData: &models.JSONB{
			"level_price":       level.Price,
			"level_distance":    distance,
			"level_strength":    level.Strength,
			"breakout_price":    price,
			"level_description": desc,
		},
		Status:           models.SignalStatusPending,
		NotificationSent: false,
		CreatedAt:       time.Now(),
		KlineTime:        ptrTime(kline.OpenTime),
	}
}

// ==================== 工具函数 ====================

// calcATR 计算 ATR（Average True Range），返回绝对值和占价格的百分比
func (s *KeyLevelStrategy) calcATR(klines []models.Kline, period int) (atr float64, atrPercent float64) {
	if period < 5 {
		period = 14
	}
	if len(klines) < period+1 {
		return 0, 0
	}
	recent := klines[len(klines)-period-1:]
	var trSum float64
	for i := 1; i < len(recent); i++ {
		tr := math.Max(
			recent[i].HighPrice-recent[i].LowPrice,
			math.Max(
				math.Abs(recent[i].HighPrice-recent[i-1].ClosePrice),
				math.Abs(recent[i].LowPrice-recent[i-1].ClosePrice),
			),
		)
		trSum += tr
	}
	atr = trSum / float64(period)
	latestClose := klines[len(klines)-1].ClosePrice
	if latestClose > 0 {
		atrPercent = (atr / latestClose) * 100
	}
	return
}

// checkVolumeConfirm 量能确认：突破K线成交量需 > 20周期均量 * 1.5
func (s *KeyLevelStrategy) checkVolumeConfirm(klines []models.Kline, latestIdx int) bool {
	if latestIdx < 20 {
		return true
	}
	var sumVol float64
	for i := latestIdx - 20; i < latestIdx; i++ {
		sumVol += klines[i].Volume
	}
	avgVol := sumVol / 20.0
	if avgVol == 0 {
		return true
	}
	return klines[latestIdx].Volume > avgVol*1.5
}

// strengthToInt 将 strength 字符串转为信号强度（1-5）
func strengthToInt(s string) int {
	switch s {
	case models.LevelStrengthStrong:
		return 5
	case models.LevelStrengthMid:
		return 3
	case models.LevelStrengthWeak:
		return 1
	default:
		return 3
	}
}
