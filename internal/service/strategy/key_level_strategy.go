package strategy

import (
	"fmt"
	"sort"
	"time"

	"github.com/smallfire/starfire/internal/config"
	"github.com/smallfire/starfire/internal/models"
)

// KeyLevelStrategy 阻力支撑策略
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

func (s *KeyLevelStrategy) Analyze(symbolID int, symbolCode, period string, klines []models.Kline) ([]models.Signal, error) {
	if len(klines) < s.config.LookbackKlines {
		return nil, nil
	}

	var signals []models.Signal

	// 1. 识别关键价位
	levels := s.identifyKeyLevels(klines)

	// 2. 保存/更新价位
	for _, level := range levels {
		existing, _ := s.deps.LevelRepo.FindActive(symbolID, period, level.LevelSubtype)
		if existing != nil {
			// 更新触及次数
			existing.KlinesCount++
			s.deps.LevelRepo.Update(existing)
		} else {
			s.deps.LevelRepo.Create(level)
		}
	}

	// 3. 检查突破
	latestKline := klines[len(klines)-1]
	activeLevels, _ := s.deps.LevelRepo.GetActive(symbolID, period)

	// 同一根 K 线只取被突破的距当前价格最近的价位，避免重复信号
	var closestBrokenResistance *models.KeyLevel
	var closestBrokenSupport *models.KeyLevel

	for _, level := range activeLevels {
		sig := s.checkLevelBreak(symbolID, *level, latestKline)
		if sig != nil {
			// 标记所有被突破的价位
			level.Broken = true
			level.BrokenAt = &latestKline.OpenTime
			price := latestKline.ClosePrice
			level.BrokenPrice = &price
			dir := "up"
			if level.LevelType == "support" {
				dir = "down"
			}
			level.BrokenDirection = &dir
			s.deps.LevelRepo.Update(level)

			// 只保留距当前价格最近的
			if level.LevelType == "resistance" {
				if closestBrokenResistance == nil || level.Price > closestBrokenResistance.Price {
					closestBrokenResistance = level
				}
			} else {
				if closestBrokenSupport == nil || level.Price < closestBrokenSupport.Price {
					closestBrokenSupport = level
				}
			}
		}
	}

	// 生成信号：只产生最近的一个阻力突破和一个支撑跌破信号
	if closestBrokenResistance != nil {
		if sig := s.checkLevelBreak(symbolID, *closestBrokenResistance, latestKline); sig != nil {
			signals = append(signals, *sig)
		}
	}
	if closestBrokenSupport != nil {
		if sig := s.checkLevelBreak(symbolID, *closestBrokenSupport, latestKline); sig != nil {
			signals = append(signals, *sig)
		}
	}

	return signals, nil
}

// identifyKeyLevels 识别关键价位
func (s *KeyLevelStrategy) identifyKeyLevels(klines []models.Kline) []*models.KeyLevel {
	var levels []*models.KeyLevel
	symbolID := klines[0].SymbolID
	period := klines[0].Period

	// 找出最近的高点和低点
	recentKlines := klines[len(klines)-s.config.LookbackKlines:]

	var highs, lows []float64
	for _, k := range recentKlines {
		highs = append(highs, k.HighPrice)
		lows = append(lows, k.LowPrice)
	}

	sort.Float64s(highs)
	sort.Float64s(lows)

	// 取最近4个高点和低点
	if len(highs) >= 4 {
		// 找最高的几个价格作为阻力位
		levels = append(levels, &models.KeyLevel{
			SymbolID:     symbolID,
			Period:       period,
			LevelType:    "resistance",
			LevelSubtype: "current_high",
			Price:        highs[len(highs)-1],
			Broken:       false,
			KlinesCount:  1,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		})

		levels = append(levels, &models.KeyLevel{
			SymbolID:     symbolID,
			Period:       period,
			LevelType:    "resistance",
			LevelSubtype: "prev_high",
			Price:        highs[len(highs)-2],
			Broken:       false,
			KlinesCount:  1,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		})
	}

	if len(lows) >= 4 {
		levels = append(levels, &models.KeyLevel{
			SymbolID:     symbolID,
			Period:       period,
			LevelType:    "support",
			LevelSubtype: "current_low",
			Price:        lows[0],
			Broken:       false,
			KlinesCount:  1,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		})

		levels = append(levels, &models.KeyLevel{
			SymbolID:     symbolID,
			Period:       period,
			LevelType:    "support",
			LevelSubtype: "prev_low",
			Price:        lows[1],
			Broken:       false,
			KlinesCount:  1,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		})
	}

	return levels
}

// checkLevelBreak 检查价位突破
func (s *KeyLevelStrategy) checkLevelBreak(symbolID int, level models.KeyLevel, kline models.Kline) *models.Signal {
	price := kline.ClosePrice
	levelPrice := level.Price
	threshold := s.config.LevelDistance / 100 * levelPrice

	// 价位子类型的中文名映射
	subtypeNames := map[string]string{
		"current_high": "近期高点",
		"prev_high":    "前期高点",
		"current_low":  "近期低点",
		"prev_low":     "前期低点",
	}
	subtypeName := subtypeNames[level.LevelSubtype]
	if subtypeName == "" {
		subtypeName = level.LevelSubtype
	}

	if level.LevelType == "resistance" {
		if price > levelPrice+threshold {
			distance := (price - level.Price) / level.Price * 100
			signalData := &models.JSONB{
				"level_id":         level.ID,
				"level_type":       level.LevelType,
				"level_subtype":    level.LevelSubtype,
				"level_price":      level.Price,
				"level_distance":   distance,
				"klines_count":     level.KlinesCount,
				"breakout_price":   price,
				"level_description": fmt.Sprintf("%s阻力位，触及%d次，距突破%.2f%%", subtypeName, level.KlinesCount, distance),
			}

			return &models.Signal{
				SymbolID:         symbolID,
				SignalType:       models.SignalTypeResistanceBreak,
				SourceType:       models.SourceTypeKeyLevel,
				Direction:        "long",
				Strength:         level.KlinesCount,
				Price:            price,
				StopLossPrice:    &level.Price,
				Period:           kline.Period,
				SignalData:       signalData,
				Status:           models.SignalStatusPending,
				NotificationSent: false,
				CreatedAt:        time.Now(),
				KlineTime:        ptrTime(kline.CloseTime),
			}
		}
	} else if level.LevelType == "support" {
		if price < levelPrice-threshold {
			distance := (level.Price - price) / level.Price * 100
			signalData := &models.JSONB{
				"level_id":         level.ID,
				"level_type":       level.LevelType,
				"level_subtype":    level.LevelSubtype,
				"level_price":      level.Price,
				"level_distance":   distance,
				"klines_count":     level.KlinesCount,
				"breakout_price":   price,
				"level_description": fmt.Sprintf("%s支撑位，触及%d次，距跌破%.2f%%", subtypeName, level.KlinesCount, distance),
			}

			return &models.Signal{
				SymbolID:         symbolID,
				SignalType:       models.SignalTypeSupportBreak,
				SourceType:       models.SourceTypeKeyLevel,
				Direction:        "short",
				Strength:         level.KlinesCount,
				Price:            price,
				StopLossPrice:    &level.Price,
				Period:           kline.Period,
				SignalData:       signalData,
				Status:           models.SignalStatusPending,
				NotificationSent: false,
				CreatedAt:        time.Now(),
				KlineTime:        ptrTime(kline.CloseTime),
			}
		}
	}

	return nil
}
