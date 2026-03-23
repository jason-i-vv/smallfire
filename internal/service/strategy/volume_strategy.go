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
func (s *VolumePriceStrategy) Type() string        { return "volume" }
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

		expireTime := time.Now().Add(6 * time.Hour)

		return &models.Signal{
			SymbolID:         symbolID,
			SignalType:       signalType,
			SourceType:       models.SourceTypeVolume,
			Direction:        direction,
			Strength:         2,
			Price:            latest.ClosePrice,
			Period:           latest.Period,
			SignalData:       &models.JSONB{},
			Status:           models.SignalStatusPending,
			ExpiredAt:        &expireTime,
			NotificationSent: false,
			CreatedAt:        time.Now(),
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

		expireTime := time.Now().Add(6 * time.Hour)

		return &models.Signal{
			SymbolID:         symbolID,
			SignalType:       signalType,
			SourceType:       models.SourceTypeVolume,
			Direction:        direction,
			Strength:         2,
			Price:            latest.ClosePrice,
			Period:           latest.Period,
			SignalData:       &models.JSONB{},
			Status:           models.SignalStatusPending,
			ExpiredAt:        &expireTime,
			NotificationSent: false,
			CreatedAt:        time.Now(),
		}
	}

	return nil
}
