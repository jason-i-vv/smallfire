package strategy

import (
	"time"

	"github.com/smallfire/starfire/internal/config"
	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/internal/service/trend"
)

// TrendStrategy 趋势策略
type TrendStrategy struct {
	config config.TrendStrategyConfig
	deps   Dependency
}

// NewTrendStrategy 创建趋势策略实例
func NewTrendStrategy(cfg config.TrendStrategyConfig, deps Dependency) Strategy {
	return &TrendStrategy{
		config: cfg,
		deps:   deps,
	}
}

func (s *TrendStrategy) Name() string        { return "trend_strategy" }
func (s *TrendStrategy) Type() string        { return "trend" }
func (s *TrendStrategy) Enabled() bool       { return s.config.Enabled }
func (s *TrendStrategy) Config() interface{} { return s.config }

func (s *TrendStrategy) Analyze(symbolID int, symbolCode, period string, klines []models.Kline) ([]models.Signal, error) {
	if len(klines) < 90 {
		return nil, nil
	}

	var signals []models.Signal

	// 1. 确定趋势（使用统一的趋势计算）
	trendModel := s.determineTrend(symbolID, period, klines)

	// 2. 检查趋势变化
	activeTrend, _ := s.deps.TrendRepo.GetActive(symbolID, period)

	if activeTrend == nil {
		s.deps.TrendRepo.Create(trendModel)
	} else if trendModel.TrendType != activeTrend.TrendType {
		activeTrend.Status = models.TrendStatusEnded
		activeTrend.EndTime = &klines[len(klines)-1].OpenTime
		s.deps.TrendRepo.Update(activeTrend)

		s.deps.TrendRepo.Create(trendModel)
	} else {
		activeTrend.Strength = trendModel.Strength
		activeTrend.EMAShort = trendModel.EMAShort
		activeTrend.EMAMedium = trendModel.EMAMedium
		activeTrend.EMALong = trendModel.EMALong
		s.deps.TrendRepo.Update(activeTrend)

		if sig := s.checkRetracement(activeTrend, klines); sig != nil {
			signals = append(signals, *sig)
		}
	}

	return signals, nil
}

// determineTrend 确定趋势状态，使用统一的趋势计算工具
func (s *TrendStrategy) determineTrend(symbolID int, period string, klines []models.Kline) *models.Trend {
	lastKline := klines[len(klines)-1]

	var emaShort, emaMedium, emaLong float64
	if lastKline.EMAShort != nil {
		emaShort = *lastKline.EMAShort
	}
	if lastKline.EMAMedium != nil {
		emaMedium = *lastKline.EMAMedium
	}
	if lastKline.EMALong != nil {
		emaLong = *lastKline.EMALong
	}

	// 使用共享趋势计算，优先 EMA，不可用时从 K 线计算
	trendType, strength := trend.CalculateFromKlines(klines)

	return &models.Trend{
		SymbolID:  symbolID,
		Period:    period,
		TrendType: trendType,
		Strength:  strength,
		EMAShort:  emaShort,
		EMAMedium: emaMedium,
		EMALong:   emaLong,
		StartTime: klines[0].OpenTime,
		Status:    models.TrendStatusActive,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// checkRetracement 检查趋势回撤信号
func (s *TrendStrategy) checkRetracement(trend *models.Trend, klines []models.Kline) *models.Signal {
	lastKline := klines[len(klines)-1]
	price := lastKline.ClosePrice

	var retracementPct float64

	switch trend.Strength {
	case 3:
		if trend.TrendType == models.TrendTypeBullish && trend.EMALong != 0 {
			retracementPct = (trend.EMALong - price) / trend.EMALong
		} else if trend.TrendType == models.TrendTypeBearish && trend.EMALong != 0 {
			retracementPct = (price - trend.EMALong) / trend.EMALong
		}
	case 2:
		if trend.TrendType == models.TrendTypeBullish && trend.EMAMedium != 0 {
			retracementPct = (trend.EMAMedium - price) / trend.EMAMedium
		} else if trend.TrendType == models.TrendTypeBearish && trend.EMAMedium != 0 {
			retracementPct = (price - trend.EMAMedium) / trend.EMAMedium
		}
	default:
		return nil
	}

	if retracementPct > 0.01 && retracementPct < 0.05 {
		direction := "long"
		if trend.TrendType == models.TrendTypeBearish {
			direction = "short"
		}

		expireTime := time.Now().Add(12 * time.Hour)

		return &models.Signal{
			SignalType:       models.SignalTypeTrendRetracement,
			SourceType:       models.SourceTypeTrend,
			Direction:        direction,
			Strength:         trend.Strength,
			Price:            price,
			Period:           lastKline.Period,
			SignalData:       &models.JSONB{},
			Status:           models.SignalStatusPending,
			ExpiredAt:        &expireTime,
			NotificationSent: false,
			CreatedAt:        time.Now(),
			KlineTime:        ptrTime(lastKline.OpenTime),
		}
	}

	return nil
}
