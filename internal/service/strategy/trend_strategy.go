package strategy

import (
	"math"
	"time"

	"github.com/smallfire/starfire/internal/config"
	"github.com/smallfire/starfire/internal/models"
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

	// 1. 确定趋势
	trend := s.determineTrend(klines)

	// 2. 检查趋势变化
	activeTrend, _ := s.deps.TrendRepo.GetActive(symbolID, period)

	if activeTrend == nil {
		// 新趋势
		s.deps.TrendRepo.Create(trend)
	} else if trend.TrendType != activeTrend.TrendType {
		// 趋势反转
		activeTrend.Status = models.TrendStatusEnded
		activeTrend.EndTime = &klines[len(klines)-1].OpenTime
		s.deps.TrendRepo.Update(activeTrend)

		// 生成反转信号
		sig := s.createReversalSignal(trend, klines[len(klines)-1])
		signals = append(signals, *sig)

		// 创建新趋势
		s.deps.TrendRepo.Create(trend)
	} else {
		// 更新趋势
		activeTrend.Strength = trend.Strength
		activeTrend.EMAShort = trend.EMAShort
		activeTrend.EMAMedium = trend.EMAMedium
		activeTrend.EMALong = trend.EMALong
		s.deps.TrendRepo.Update(activeTrend)

		// 检查趋势回撤信号
		if sig := s.checkRetracement(activeTrend, klines); sig != nil {
			signals = append(signals, *sig)
		}
	}

	return signals, nil
}

// determineTrend 确定趋势状态
func (s *TrendStrategy) determineTrend(klines []models.Kline) *models.Trend {
	// 获取最新的EMA值
	lastKline := klines[len(klines)-1]

	var emaShort, emaMedium, emaLong float64
	var trendType string
	var strength int

	if lastKline.EMAShort != nil {
		emaShort = *lastKline.EMAShort
	}
	if lastKline.EMAMedium != nil {
		emaMedium = *lastKline.EMAMedium
	}
	if lastKline.EMALong != nil {
		emaLong = *lastKline.EMALong
	}

	if emaShort > emaMedium && emaMedium > emaLong {
		trendType = models.TrendTypeBullish
	} else if emaShort < emaMedium && emaMedium < emaLong {
		trendType = models.TrendTypeBearish
	} else {
		trendType = models.TrendTypeSideways
	}

	// 计算趋势强度（基于EMA间距）
	shortMedGap := math.Abs(emaShort-emaMedium) / emaMedium
	medLongGap := math.Abs(emaMedium-emaLong) / emaLong

	if shortMedGap > 0.01 && medLongGap > 0.02 {
		strength = 3
	} else if shortMedGap > 0.005 && medLongGap > 0.01 {
		strength = 2
	} else {
		strength = 1
	}

	return &models.Trend{
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

// createReversalSignal 创建趋势反转信号
func (s *TrendStrategy) createReversalSignal(trend *models.Trend, kline models.Kline) *models.Signal {
	direction := "long"
	if trend.TrendType == models.TrendTypeBearish {
		direction = "short"
	}

	signalType := models.SignalTypeTrendReversal
	price := kline.ClosePrice

	// 计算止盈止损（基于长期趋势）
	var stopLoss, target float64
	if trend.EMALong != 0 {
		if trend.TrendType == models.TrendTypeBullish {
			stopLoss = trend.EMALong * 0.995    // 0.5% below EMA90
			target = price + (price-stopLoss)*3 // 3倍风险收益比
		} else {
			stopLoss = trend.EMALong * 1.005    // 0.5% above EMA90
			target = price - (stopLoss-price)*3 // 3倍风险收益比
		}
	}

	expireTime := time.Now().Add(24 * time.Hour)

	return &models.Signal{
		SignalType:       signalType,
		SourceType:       models.SourceTypeTrend,
		Direction:        direction,
		Strength:         trend.Strength,
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

// checkRetracement 检查趋势回撤信号
func (s *TrendStrategy) checkRetracement(trend *models.Trend, klines []models.Kline) *models.Signal {
	lastKline := klines[len(klines)-1]
	price := lastKline.ClosePrice

	// 计算回撤幅度
	var retracementPct float64

	switch trend.Strength {
	case 3: // 长期均线回撤
		if trend.TrendType == models.TrendTypeBullish && trend.EMALong != 0 {
			retracementPct = (trend.EMALong - price) / trend.EMALong
		} else if trend.TrendType == models.TrendTypeBearish && trend.EMALong != 0 {
			retracementPct = (price - trend.EMALong) / trend.EMALong
		}
	case 2: // 中期均线回撤
		if trend.TrendType == models.TrendTypeBullish && trend.EMAMedium != 0 {
			retracementPct = (trend.EMAMedium - price) / trend.EMAMedium
		} else if trend.TrendType == models.TrendTypeBearish && trend.EMAMedium != 0 {
			retracementPct = (price - trend.EMAMedium) / trend.EMAMedium
		}
	default:
		return nil
	}

	// 回撤超过1%触发信号
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
		}
	}

	return nil
}
