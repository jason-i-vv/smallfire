package strategy

import (
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

	for _, level := range activeLevels {
		if sig := s.checkLevelBreak(symbolID, *level, latestKline); sig != nil {
			signals = append(signals, *sig)
			// 更新价位状态
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

	if level.LevelType == "resistance" {
		if price > levelPrice+threshold {
			return &models.Signal{
				SymbolID:         symbolID,
				SignalType:       models.SignalTypeResistanceBreak,
				SourceType:       models.SourceTypeKeyLevel,
				Direction:        "long",
				Strength:         level.KlinesCount, // 触及次数越多信号越强
				Price:            price,
				StopLossPrice:    &level.Price,
				Period:           kline.Period,
				Status:           models.SignalStatusPending,
				NotificationSent: false,
				CreatedAt:        time.Now(),
			}
		}
	} else if level.LevelType == "support" {
		if price < levelPrice-threshold {
			return &models.Signal{
				SymbolID:         symbolID,
				SignalType:       models.SignalTypeSupportBreak,
				SourceType:       models.SourceTypeKeyLevel,
				Direction:        "short",
				Strength:         level.KlinesCount,
				Price:            price,
				StopLossPrice:    &level.Price,
				Period:           kline.Period,
				Status:           models.SignalStatusPending,
				NotificationSent: false,
				CreatedAt:        time.Now(),
			}
		}
	}

	return nil
}
