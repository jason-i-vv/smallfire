package strategy

import (
	"fmt"
	"math"
	"time"

	"github.com/smallfire/starfire/internal/config"
	"github.com/smallfire/starfire/internal/models"
)

// KeyLevelStrategy 阻力支撑策略
// 使用波峰波谷（swing point）识别关键价位，通过触及次数评估信号强度
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

// keyLevelSwingPoint 关键位策略的波峰波谷
type keyLevelSwingPoint struct {
	Index int
	Type  int // 0: 波峰(阻力), 1: 波谷(支撑)
	Price float64
	Time  time.Time
}

func (s *KeyLevelStrategy) Analyze(symbolID int, symbolCode, period string, klines []models.Kline) ([]models.Signal, error) {
	if len(klines) < s.config.LookbackKlines {
		return nil, nil
	}

	latestKline := klines[len(klines)-1]
	latestPrice := latestKline.ClosePrice
	latestIdx := len(klines) - 1

	// 1. 检测波峰波谷，识别新关键位
	swings := s.detectSwingPoints(klines)

	// 2. 将新检测到的波峰波谷入库（去重：价格在 0.1% 以内视为同一价位）
	for _, sw := range swings {
		levelType := models.LevelTypeResistance
		subtype := "swing_high"
		if sw.Type == 1 {
			levelType = models.LevelTypeSupport
			subtype = "swing_low"
		}

		// 检查是否已存在相近价位
		existing, _ := s.deps.LevelRepo.GetActive(symbolID, period)
		exists := false
		for _, l := range existing {
			if l.LevelType == levelType && math.Abs(l.Price-sw.Price)/sw.Price < 0.001 {
				// 存在相近价位，增加触及次数
				l.KlinesCount++
				s.deps.LevelRepo.Update(l)
				exists = true
				break
			}
		}
		if !exists {
			// 有效期 = lookbackKlines * 2 根K线的时长
				validUntil := s.getValidUntil(period)
				s.deps.LevelRepo.Create(&models.KeyLevel{
				SymbolID:     symbolID,
				Period:       period,
				LevelType:    levelType,
				LevelSubtype: subtype,
				Price:        sw.Price,
				Broken:       false,
				KlinesCount:  1,
				ValidUntil:   validUntil,
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
				})
		}
	}

	// 3. 更新已有价位的触及次数
	s.updateTouchCounts(symbolID, period, klines, latestIdx)

	// 4. 检查突破
	activeLevels, _ := s.deps.LevelRepo.GetActive(symbolID, period)
	threshold := s.config.LevelDistance / 100.0

	var signals []models.Signal

	// 阻力位突破（向上）：只取最近一个被突破的阻力位
	var closestBrokenResistance *models.KeyLevel
	for _, level := range activeLevels {
		if level.Broken || level.LevelType != models.LevelTypeResistance {
			continue
		}
		// 价位至少经过 minBreakoutAge 次触及验证后才允许产生突破信号
		if s.config.MinBreakoutAge > 0 && level.KlinesCount < s.config.MinBreakoutAge {
			continue
		}
		breakoutPrice := level.Price * (1 + threshold)
		if latestPrice > breakoutPrice {
			level.Broken = true
			level.BrokenAt = &latestKline.OpenTime
			level.BrokenPrice = &latestPrice
			dir := models.LevelBreakDirectionUp
			level.BrokenDirection = &dir
			s.deps.LevelRepo.Update(level)

			if closestBrokenResistance == nil || level.Price < closestBrokenResistance.Price {
				closestBrokenResistance = level
			}
		}
	}

	if closestBrokenResistance != nil {
		signals = append(signals, s.createBreakSignal(symbolID, *closestBrokenResistance, latestKline, "long"))
	}

	// 支撑位突破（向下）：只取最近一个被突破的支撑位
	var closestBrokenSupport *models.KeyLevel
	for _, level := range activeLevels {
		if level.Broken || level.LevelType != models.LevelTypeSupport {
			continue
		}
		// 价位至少经过 minBreakoutAge 次触及验证后才允许产生突破信号
		if s.config.MinBreakoutAge > 0 && level.KlinesCount < s.config.MinBreakoutAge {
			continue
		}
		breakoutPrice := level.Price * (1 - threshold)
		if latestPrice < breakoutPrice {
			level.Broken = true
			level.BrokenAt = &latestKline.OpenTime
			level.BrokenPrice = &latestPrice
			dir := models.LevelBreakDirectionDown
			level.BrokenDirection = &dir
			s.deps.LevelRepo.Update(level)

			if closestBrokenSupport == nil || level.Price > closestBrokenSupport.Price {
				closestBrokenSupport = level
			}
		}
	}

	if closestBrokenSupport != nil {
		signals = append(signals, s.createBreakSignal(symbolID, *closestBrokenSupport, latestKline, "short"))
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

			// 距离相近时，取触及次数更高的一方
			if longDist > 0 && shortDist > 0 && math.Abs(longDist-shortDist)/math.Max(longDist, shortDist) < 0.3 {
				longTouch := int((*longSig.SignalData)["klines_count"].(float64))
				shortTouch := int((*shortSig.SignalData)["klines_count"].(float64))
				if longTouch >= shortTouch {
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

// getValidUntil 计算价位有效期（lookbackKlines * 2 根K线的时长）
func (s *KeyLevelStrategy) getValidUntil(period string) *time.Time {
	var duration time.Duration
	switch period {
	case "1m":
		duration = time.Minute
	case "5m":
		duration = 5 * time.Minute
	case "15m":
		duration = 15 * time.Minute
	case "30m":
		duration = 30 * time.Minute
	case "1h":
		duration = time.Hour
	case "4h":
		duration = 4 * time.Hour
	case "1d":
		duration = 24 * time.Hour
	default:
		duration = time.Hour
	}
	validUntil := time.Now().Add(duration * time.Duration(s.config.LookbackKlines*2))
	return &validUntil
}

// detectSwingPoints 检测波峰波谷
// 波峰 = 阻力位候选，波谷 = 支撑位候选
func (s *KeyLevelStrategy) detectSwingPoints(klines []models.Kline) []keyLevelSwingPoint {
	var swings []keyLevelSwingPoint
	lookback := 3 // 波峰波谷检测回溯数

	// 只检测最近 lookbackKlines 根 K 线中的 swing points
	startIdx := 0
	if len(klines) > s.config.LookbackKlines {
		startIdx = len(klines) - s.config.LookbackKlines
	}

	for i := startIdx + lookback; i < len(klines)-lookback; i++ {
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
			swings = append(swings, keyLevelSwingPoint{
				Index: i,
				Type:  0, // 波峰
				Price: currHigh,
				Time:  klines[i].OpenTime,
			})
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
			swings = append(swings, keyLevelSwingPoint{
				Index: i,
				Type:  1, // 波谷
				Price: currLow,
				Time:  klines[i].OpenTime,
			})
		}
	}

	return swings
}

// updateTouchCounts 更新已有价位的触及次数（只检查最新1根K线，与回测一致）
// "触及"定义为：K 线的影线触达该价位但实体未收过价位
func (s *KeyLevelStrategy) updateTouchCounts(symbolID int, period string, klines []models.Kline, latestIdx int) {
	activeLevels, _ := s.deps.LevelRepo.GetActive(symbolID, period)
	k := klines[latestIdx]

	for _, level := range activeLevels {
		if level.Broken {
			continue
		}
		tolerance := level.Price * 0.003 // 0.3% 容差视为触及

		switch level.LevelType {
		case models.LevelTypeResistance:
			// 影线触达阻力位但收盘未收过（实体仍在阻力位下方）
			if k.HighPrice >= level.Price-tolerance && k.ClosePrice <= level.Price {
				level.KlinesCount++
				s.deps.LevelRepo.Update(level)
			}
		case models.LevelTypeSupport:
			// 影线触达支撑位但收盘未收过（实体仍在支撑位上方）
			if k.LowPrice <= level.Price+tolerance && k.ClosePrice >= level.Price {
				level.KlinesCount++
				s.deps.LevelRepo.Update(level)
			}
		}
	}
}

// createBreakSignal 创建突破信号
func (s *KeyLevelStrategy) createBreakSignal(symbolID int, level models.KeyLevel, kline models.Kline, direction string) models.Signal {
	price := kline.ClosePrice
	distance := math.Abs(price-level.Price) / level.Price * 100

	levelLabel := "阻力位"
	actionLabel := "突破"
	if level.LevelType == models.LevelTypeSupport {
		levelLabel = "支撑位"
		actionLabel = "跌破"
	}

	signalType := models.SignalTypeResistanceBreak
	if level.LevelType == models.LevelTypeSupport {
		signalType = models.SignalTypeSupportBreak
	}

	// 强度基于触及次数：触及越多，信号越强
	strength := level.KlinesCount
	if strength > 5 {
		strength = 5
	}
	if strength < 1 {
		strength = 1
	}

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
			"level_id":         level.ID,
			"level_type":       level.LevelType,
			"level_price":      level.Price,
			"level_distance":   distance,
			"klines_count":     level.KlinesCount,
			"breakout_price":   price,
			"level_description": fmt.Sprintf("%s%s，触及%d次，距%s%.2f%%", actionLabel, levelLabel, level.KlinesCount, actionLabel, distance),
		},
		Status:           models.SignalStatusPending,
		NotificationSent: false,
		CreatedAt:        time.Now(),
		KlineTime:        ptrTime(kline.OpenTime),
	}
}
