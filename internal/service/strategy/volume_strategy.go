package strategy

import (
	"sync"
	"time"

	"github.com/smallfire/starfire/internal/config"
	"github.com/smallfire/starfire/internal/models"
)

// symbolCooldown 标的级别冷却状态
type symbolCooldown struct {
	lastPriceKlineTime  time.Time
	lastVolumeKlineTime time.Time
	lastVolumeAvgBase   float64
}

// VolumePriceStrategy 量价异常策略
type VolumePriceStrategy struct {
	config config.VolumePriceStrategyConfig
	deps   Dependency

	// 冷却机制：每个标的独立冷却（key=symbolID）
	mu       sync.Mutex
	cooldowns map[int]*symbolCooldown
}

// NewVolumePriceStrategy 创建量价异常策略实例
func NewVolumePriceStrategy(cfg config.VolumePriceStrategyConfig, deps Dependency) Strategy {
	return &VolumePriceStrategy{
		config:    cfg,
		deps:      deps,
		cooldowns: make(map[int]*symbolCooldown),
	}
}

func (s *VolumePriceStrategy) Name() string        { return "volume_price_strategy" }
func (s *VolumePriceStrategy) Type() string        { return "volume" }
func (s *VolumePriceStrategy) Enabled() bool       { return s.config.Enabled }
func (s *VolumePriceStrategy) Config() interface{} { return s.config }

func (s *VolumePriceStrategy) Analyze(symbolID int, symbolCode, period string, klines []models.Kline) ([]models.Signal, error) {
	if len(klines) < s.config.LookbackKlines {
		return nil, nil
	}

	latestKline := klines[len(klines)-1]
	historicalKlines := klines[len(klines)-s.config.LookbackKlines : len(klines)-1]

	// 计算 ATR（供两个子函数共用）
	atrPeriod := 14
	if s.config.ATRPeriod > 0 {
		atrPeriod = int(s.config.ATRPeriod)
	}
	atr := CalculateATR(klines, atrPeriod)

	var signals []models.Signal

	// 1. 检查价格波动异常（带冷却）
	if sig := s.checkPriceAnomaly(symbolID, latestKline, historicalKlines, atr); sig != nil {
		signals = append(signals, *sig)
	}

	// 2. 检查成交量异常（带冷却）
	if sig := s.checkVolumeAnomaly(symbolID, latestKline, historicalKlines, atr); sig != nil {
		signals = append(signals, *sig)
	}

	// 同一根K线只保留强度最高的信号
	if len(signals) > 1 {
		best := signals[0]
		for i := 1; i < len(signals); i++ {
			if signals[i].Strength > best.Strength {
				best = signals[i]
			}
		}
		signals = []models.Signal{best}
	}

	return signals, nil
}

// cooldownDuration 返回信号冷却时间
func (s *VolumePriceStrategy) cooldownDuration() time.Duration {
	// 默认 1 小时冷却
	minutes := 60
	if minutes < 10 {
		minutes = 10
	}
	return time.Duration(minutes) * time.Minute
}

// getCooldown 获取标的的冷却状态（线程安全）
func (s *VolumePriceStrategy) getCooldown(symbolID int) *symbolCooldown {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cooldowns[symbolID] == nil {
		s.cooldowns[symbolID] = &symbolCooldown{}
	}
	return s.cooldowns[symbolID]
}

// checkPriceAnomaly 检查价格波动异常（带冷却）
func (s *VolumePriceStrategy) checkPriceAnomaly(symbolID int, latest models.Kline, historical []models.Kline, atr float64) *models.Signal {
	cd := s.getCooldown(symbolID)

	s.mu.Lock()
	if !cd.lastPriceKlineTime.IsZero() && latest.OpenTime.Sub(cd.lastPriceKlineTime) < s.cooldownDuration() {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	// 计算历史波动幅度
	var totalVol float64
	for _, k := range historical {
		vol := (k.HighPrice - k.LowPrice) / k.ClosePrice
		totalVol += vol
	}
	avgVol := totalVol / float64(len(historical))
	threshold := avgVol * s.config.VolatilityMultiplier

	// 当前波动
	currentVol := (latest.HighPrice - latest.LowPrice) / latest.ClosePrice

	if currentVol > threshold {
		s.mu.Lock()
		cd.lastPriceKlineTime = latest.OpenTime
		s.mu.Unlock()

		direction := "long"
		if latest.ClosePrice < latest.OpenPrice {
			direction = "short"
		}

		signalType := models.SignalTypePriceSurgeUp
		if latest.ClosePrice < latest.OpenPrice {
			signalType = models.SignalTypePriceSurgeDown
		}
		priceAmplification := currentVol / avgVol
		strength := calculateStrength(priceAmplification, s.config.VolatilityMultiplier)
		expireTime := time.Now().Add(6 * time.Hour)

		// 基于 ATR 计算止盈止损
		var stopLoss, takeProfit float64
		if atr > 0 {
			stopLoss, takeProfit = CalculateSLTP(latest.ClosePrice, direction, atr, s.config.ATRMultiplier, s.config.RiskRewardRatio)
		}

		return &models.Signal{
			SymbolID:   symbolID,
			SignalType: signalType,
			SourceType: models.SourceTypeVolume,
			Direction:  direction,
			Strength:   strength,
			Price:      latest.ClosePrice,
			Period:     latest.Period,
			SignalData: &models.JSONB{
				"price_amplification": priceAmplification,
				"volatility_threshold": threshold,
				"current_volatility":  currentVol,
				"avg_volatility":       avgVol,
			},
			Status:           models.SignalStatusPending,
			ExpiredAt:        &expireTime,
			NotificationSent: false,
			CreatedAt:        time.Now(),
			KlineTime:        ptrTime(latest.OpenTime),
			StopLossPrice:    &stopLoss,
			TargetPrice:      &takeProfit,
		}
	}

	return nil
}

// checkVolumeAnomaly 检查成交量异常（带冷却 + 量能基准去重）
func (s *VolumePriceStrategy) checkVolumeAnomaly(symbolID int, latest models.Kline, historical []models.Kline, atr float64) *models.Signal {
	cd := s.getCooldown(symbolID)

	s.mu.Lock()
	// 冷却期检查：基于K线时间
	if !cd.lastVolumeKlineTime.IsZero() && latest.OpenTime.Sub(cd.lastVolumeKlineTime) < s.cooldownDuration() {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	// 计算历史平均成交量
	var totalVol float64
	for _, k := range historical {
		totalVol += k.Volume
	}
	avgVol := totalVol / float64(len(historical))
	threshold := avgVol * s.config.VolumeMultiplier

	if latest.Volume > threshold {
		s.mu.Lock()

		// 量能基准去重：如果已有基准值，当前K线必须超越基准的倍量才算有效信号
		// 冷却期过后，如果量能没有进一步增强（倍量），仍然不触发
		if cd.lastVolumeAvgBase > 0 && cd.lastVolumeKlineTime.Before(latest.OpenTime) {
			baselineThreshold := cd.lastVolumeAvgBase * s.config.VolumeMultiplier
			if latest.Volume <= baselineThreshold {
				s.mu.Unlock()
				return nil
			}
		}

		cd.lastVolumeKlineTime = latest.OpenTime
		cd.lastVolumeAvgBase = avgVol // 记录本次触发时的平均成交量作为基准
		s.mu.Unlock()

		direction := "long"
		if latest.ClosePrice < latest.OpenPrice {
			direction = "short"
		}

		priceChange := (latest.ClosePrice - latest.OpenPrice) / latest.OpenPrice
		signalType := models.SignalTypeVolumeSurge
		if priceChange > 0.01 {
			signalType = "volume_price_rise"
		} else if priceChange < -0.01 {
			signalType = "volume_price_fall"
		}

		volumeAmplification := latest.Volume / avgVol
		strength := calculateStrength(volumeAmplification, s.config.VolumeMultiplier)
		expireTime := time.Now().Add(6 * time.Hour)

		// 基于传入的 ATR 计算止盈止损
		var stopLoss, takeProfit float64
		if atr > 0 {
			stopLoss, takeProfit = CalculateSLTP(latest.ClosePrice, direction, atr, s.config.ATRMultiplier, s.config.RiskRewardRatio)
		}

		return &models.Signal{
			SymbolID:   symbolID,
			SignalType: signalType,
			SourceType: models.SourceTypeVolume,
			Direction:  direction,
			Strength:   strength,
			Price:      latest.ClosePrice,
			Period:     latest.Period,
			SignalData: &models.JSONB{
				"volume_ratio":          volumeAmplification,
				"volume_threshold":       threshold,
				"current_volume":         latest.Volume,
				"avg_volume":             avgVol,
				"price_change_percent":   priceChange * 100,
			},
			Status:           models.SignalStatusPending,
			ExpiredAt:        &expireTime,
			NotificationSent: false,
			CreatedAt:        time.Now(),
			KlineTime:        ptrTime(latest.OpenTime),
			StopLossPrice:    &stopLoss,
			TargetPrice:      &takeProfit,
		}
	}

	return nil
}

// calculateStrength 根据放大倍数计算信号强度
func calculateStrength(amplification, thresholdMultiplier float64) int {
	if amplification >= thresholdMultiplier*6 {
		return 5
	}
	if amplification >= thresholdMultiplier*5 {
		return 5
	}
	if amplification >= thresholdMultiplier*4 {
		return 4
	}
	if amplification >= thresholdMultiplier*3 {
		return 3
	}
	if amplification >= thresholdMultiplier*2 {
		return 2
	}
	return 1
}
