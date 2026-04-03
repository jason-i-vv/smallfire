package strategy

import (
	"time"

	"github.com/smallfire/starfire/internal/config"
	"github.com/smallfire/starfire/internal/models"
)

// VolumePriceStrategy 量价异常策略
type VolumePriceStrategy struct {
	config config.VolumePriceStrategyConfig
	deps   Dependency
}

// NewVolumePriceStrategy 创建量价异常策略实例
func NewVolumePriceStrategy(cfg config.VolumePriceStrategyConfig, deps Dependency) Strategy {
	return &VolumePriceStrategy{
		config: cfg,
		deps:   deps,
	}
}

func (s *VolumePriceStrategy) Name() string        { return "volume_price_strategy" }
func (s *VolumePriceStrategy) Type() string        { return "volume_price" }
func (s *VolumePriceStrategy) Enabled() bool       { return s.config.Enabled }
func (s *VolumePriceStrategy) Config() interface{} { return s.config }

func (s *VolumePriceStrategy) Analyze(symbolID int, symbolCode, period string, klines []models.Kline) ([]models.Signal, error) {
	if len(klines) < s.config.LookbackKlines {
		return nil, nil
	}

	latestKline := klines[len(klines)-1]
	historicalKlines := klines[len(klines)-s.config.LookbackKlines : len(klines)-1]

	var signals []models.Signal

	// 1. 检查价格波动异常
	if sig := s.checkPriceAnomaly(symbolID, latestKline, historicalKlines); sig != nil {
		signals = append(signals, *sig)
	}

	// 2. 检查成交量异常
	if sig := s.checkVolumeAnomaly(symbolID, latestKline, historicalKlines); sig != nil {
		signals = append(signals, *sig)
	}

	return signals, nil
}

// checkPriceAnomaly 检查价格波动异常
func (s *VolumePriceStrategy) checkPriceAnomaly(symbolID int, latest models.Kline, historical []models.Kline) *models.Signal {
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
		direction := "long"
		if latest.ClosePrice < latest.OpenPrice {
			direction = "short"
		}

		signalType := models.SignalTypePriceSurge

		// 计算价格放大倍数
		priceAmplification := currentVol / avgVol

		// 根据放大倍数动态计算强度
		// 2倍触发，2-3倍为1星，3-4倍为2星，4-5倍为3星，5倍以上为4星
		strength := calculateStrength(priceAmplification, s.config.VolatilityMultiplier)

		expireTime := time.Now().Add(6 * time.Hour)

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
		}
	}

	return nil
}

// checkVolumeAnomaly 检查成交量异常
func (s *VolumePriceStrategy) checkVolumeAnomaly(symbolID int, latest models.Kline, historical []models.Kline) *models.Signal {
	// 计算历史平均成交量
	var totalVol float64
	for _, k := range historical {
		totalVol += k.Volume
	}
	avgVol := totalVol / float64(len(historical))
	threshold := avgVol * s.config.VolumeMultiplier

	if latest.Volume > threshold {
		direction := "long"
		if latest.ClosePrice < latest.OpenPrice {
			direction = "short"
		}

		// 量价齐升/齐跌判断
		priceChange := (latest.ClosePrice - latest.OpenPrice) / latest.OpenPrice
		signalType := models.SignalTypeVolumeSurge
		if priceChange > 0.01 {
			signalType = "volume_price_rise" // 量价齐升
		} else if priceChange < -0.01 {
			signalType = "volume_price_fall" // 量价齐跌
		}

		// 计算量能放大倍数
		volumeAmplification := latest.Volume / avgVol

		// 根据放大倍数动态计算强度
		strength := calculateStrength(volumeAmplification, s.config.VolumeMultiplier)

		expireTime := time.Now().Add(6 * time.Hour)

		return &models.Signal{
			SymbolID:   symbolID,
			SignalType: signalType,
			SourceType: models.SourceTypeVolume,
			Direction:  direction,
			Strength:   strength,
			Price:      latest.ClosePrice,
			Period:     latest.Period,
			SignalData: &models.JSONB{
				"volume_amplification":  volumeAmplification,
				"volume_threshold":     threshold,
				"current_volume":       latest.Volume,
				"avg_volume":           avgVol,
				"price_change_percent":  priceChange * 100,
			},
			Status:           models.SignalStatusPending,
			ExpiredAt:        &expireTime,
			NotificationSent: false,
			CreatedAt:        time.Now(),
			KlineTime:        ptrTime(latest.OpenTime),
		}
	}

	return nil
}

// calculateStrength 根据放大倍数计算信号强度
// thresholdMultiplier: 触发阈值倍数（用于判断是否触发）
// amplification: 实际放大倍数
func calculateStrength(amplification, thresholdMultiplier float64) int {
	// 基准强度为1星（刚好触发）
	// 每超过1个阈值倍数，增加1星强度
	// 2倍阈值 = 1星
	// 3倍阈值 = 2星
	// 4倍阈值 = 3星
	// 5倍阈值 = 4星
	// 6倍及以上 = 5星

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
