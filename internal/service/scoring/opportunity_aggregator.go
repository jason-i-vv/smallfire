package scoring

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/internal/repository"
	"go.uber.org/zap"
)

// SignalValidityConfig 信号有效期配置（K线数量）
type SignalValidityConfig struct {
	WickKlines        int `mapstructure:"wick_klines"`        // 引线信号有效期（K线数）
	CandlestickKlines int `mapstructure:"candlestick_klines"` // K线形态有效期
	KeyLevelKlines    int `mapstructure:"key_level_klines"`   // 关键位有效期
	VolumeKlines      int `mapstructure:"volume_klines"`      // 量价信号有效期
}

// DefaultValidityConfig 默认有效期配置
var DefaultValidityConfig = SignalValidityConfig{
	WickKlines:        5,
	CandlestickKlines: 3,
	KeyLevelKlines:    20,
	VolumeKlines:      15,
}

// OpportunityNotifier 交易机会通知接口
type OpportunityNotifier interface {
	SendOpportunity(opp *models.TradingOpportunity) error
}

// OpportunityHandler 交易机会处理器接口（回调钩子）
type OpportunityHandler interface {
	OnOpportunity(opp *models.TradingOpportunity)
}

// OpportunityAggregator 交易机会聚合器
type OpportunityAggregator struct {
	oppRepo          repository.OpportunityRepo
	signalRepo       repository.SignalRepo
	statsRepo        repository.SignalTypeStatsRepo
	trackRepo        repository.TradeTrackRepo
	symbolRepo       repository.SymbolRepo
	scorer           *SignalScorer
	notifier         OpportunityNotifier
	handlers         []OpportunityHandler
	validity         SignalValidityConfig
	minScoreToCreate int
	minScoreToNotify int
	expireAfterNoNew time.Duration
	logger           *zap.Logger
	mu               sync.Mutex // 防止并发 find-or-create 竞态
}

// NewOpportunityAggregator 创建聚合器
func NewOpportunityAggregator(
	oppRepo repository.OpportunityRepo,
	signalRepo repository.SignalRepo,
	statsRepo repository.SignalTypeStatsRepo,
	trackRepo repository.TradeTrackRepo,
	symbolRepo repository.SymbolRepo,
	scorer *SignalScorer,
	validity SignalValidityConfig,
	notifier OpportunityNotifier,
	logger *zap.Logger,
	minScoreToNotify int,
) *OpportunityAggregator {
	if minScoreToNotify <= 0 {
		minScoreToNotify = 60
	}
	return &OpportunityAggregator{
		oppRepo:          oppRepo,
		signalRepo:       signalRepo,
		statsRepo:        statsRepo,
		trackRepo:        trackRepo,
		symbolRepo:       symbolRepo,
		scorer:           scorer,
		notifier:         notifier,
		validity:         validity,
		minScoreToCreate: 45,
		minScoreToNotify: minScoreToNotify,
		expireAfterNoNew: 2 * time.Hour,
		logger:           logger,
	}
}

// AggregateSignals 将新产生的信号聚合到交易机会中

// AddHandler 注册交易机会处理器
func (a *OpportunityAggregator) AddHandler(handler OpportunityHandler) {
	a.handlers = append(a.handlers, handler)
}

// invokeHandlers 触发所有处理器（每个 handler 独立执行，互不影响）
func (a *OpportunityAggregator) invokeHandlers(opp *models.TradingOpportunity) {
	for _, h := range a.handlers {
		func() {
			defer func() {
				if r := recover(); r != nil {
					a.logger.Error("handler panic", zap.Any("recover", r))
				}
			}()
			h.OnOpportunity(opp)
		}()
	}
}

// 在策略运行器每次产生新信号后调用
func (a *OpportunityAggregator) AggregateSignals(newSignals []*models.Signal) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// 按币对+方向分组
	grouped := make(map[string][]*models.Signal)
	for _, sig := range newSignals {
		if sig.Status == models.SignalStatusExpired || sig.Status == models.SignalStatusAbsorbed {
			continue
		}
		key := fmt.Sprintf("%d_%s", sig.SymbolID, sig.Direction)
		grouped[key] = append(grouped[key], sig)
	}

	for _, signals := range grouped {
		if len(signals) == 0 {
			continue
		}

		first := signals[0]
		symbolID := first.SymbolID
		direction := first.Direction

		// 计算有效期
		a.setValidity(signals)

		// 检查是否已有活跃的同方向交易机会
		existing, err := a.oppRepo.GetActiveBySymbolAndDirection(symbolID, direction)
		if err != nil {
			a.logger.Error("查询交易机会失败", zap.Error(err))
			continue
		}

		if existing != nil {
			// 如果该机会已触发过交易，则过期它，让新信号走新机会
			trades, trackErr := a.trackRepo.GetByOpportunityID(existing.ID)
			if trackErr != nil {
				a.logger.Error("查询机会关联交易失败", zap.Int("opportunity_id", existing.ID), zap.Error(trackErr))
			} else if len(trades) > 0 {
				a.logger.Info("交易机会已触发过交易，过期旧机会",
					zap.Int("existing_id", existing.ID),
					zap.String("symbol", first.SymbolCode),
					zap.Int("trade_count", len(trades)),
				)
				existing.Status = models.OpportunityStatusExpired
				expiredAt := time.Now()
				existing.ExpiredAt = &expiredAt
				if err := a.oppRepo.Update(existing); err != nil {
					a.logger.Error("过期已触发交易的交易机会失败", zap.Error(err))
				}
				existing = nil
			}
		}

		if existing != nil {
			// 检查新信号与已有机会的时间跨度是否过大
			if a.isTimeGapTooLarge(existing, signals) {
				a.logger.Info("信号时间跨度过大，过期旧机会",
					zap.Int("existing_id", existing.ID),
					zap.String("symbol", first.SymbolCode),
				)
				// 过期旧机会
				existing.Status = models.OpportunityStatusExpired
				expiredAt := time.Now()
				existing.ExpiredAt = &expiredAt
				if err := a.oppRepo.Update(existing); err != nil {
					a.logger.Error("过期旧交易机会失败", zap.Error(err))
				}
				existing = nil
			}
		}

		// 获取评分上下文
		ctx := a.buildScoringContext(signals, symbolID, direction)

		if existing != nil {
			// 更新已有机会
			a.updateOpportunity(existing, signals, ctx)
		} else {
			// 创建新机会
			a.createOpportunity(signals, ctx)
		}
	}

	return nil
}

// ExpireStaleOpportunities 过期不活跃的交易机会
func (a *OpportunityAggregator) ExpireStaleOpportunities() error {
	active, err := a.oppRepo.GetActive()
	if err != nil {
		return err
	}

	now := time.Now()
	for _, opp := range active {
		// 检查最后信号时间是否超过有效期（30分钟无新信号则过期）
		if opp.LastSignalAt != nil && now.Sub(*opp.LastSignalAt) > a.expireAfterNoNew {
			opp.Status = models.OpportunityStatusExpired
			expiredAt := time.Now()
			opp.ExpiredAt = &expiredAt
			if err := a.oppRepo.Update(opp); err != nil {
				a.logger.Error("过期交易机会失败", zap.Int("id", opp.ID), zap.Error(err))
			} else {
				a.logger.Info("交易机会已过期",
					zap.Int("id", opp.ID),
					zap.String("symbol", opp.SymbolCode),
					zap.String("direction", opp.Direction),
				)
			}
		}
	}

	return nil
}

// setValidity 设置信号有效期
func (a *OpportunityAggregator) setValidity(signals []*models.Signal) {
	for _, sig := range signals {
		if sig.KlineTime == nil {
			continue
		}

		klines := 0
		switch sig.SourceType {
		case models.SourceTypeWick:
			klines = a.validity.WickKlines
		case models.SourceTypeCandlestick:
			klines = a.validity.CandlestickKlines
		case models.SourceTypeKeyLevel:
			klines = a.validity.KeyLevelKlines
		case models.SourceTypeVolume:
			klines = a.validity.VolumeKlines
		default:
			klines = 10
		}

		// 根据周期计算持续时间
		duration := klineCountToDuration(sig.Period, klines)
		validUntil := sig.KlineTime.Add(duration)
		sig.ValidUntil = &validUntil
	}
}

// buildScoringContext 构建评分上下文
func (a *OpportunityAggregator) buildScoringContext(signals []*models.Signal, symbolID int, direction string) *ScoringContext {
	ctx := &ScoringContext{}

	// 1. 策略历史胜率 - 从统计表获取
	ctx.WinRate = a.getAverageWinRate(signals)

	// 2. 多策略共识
	ctx.ConfluenceCount = len(signals)
	ctx.TotalActiveSignals = len(signals)
	if len(signals) > 0 {
		ctx.ConfluenceRatio = 1.0 // 新信号默认全部同意（后续会加入存量信号）
	}

	// 3. 信号强度 - 取最大值
	maxStrength := 0
	for _, sig := range signals {
		if sig.Strength > maxStrength {
			maxStrength = sig.Strength
		}
	}
	ctx.Strength = maxStrength

	// 4. 成交量确认 - 从信号数据提取
	ctx.VolumeRatio = a.extractVolumeRatio(signals)

	// 5. 市场状态 - 基于4h趋势计算
	regime, matchScore := a.computeRegimeMatchScore(signals, symbolID)
	ctx.RegimeMatchScore = matchScore
	ctx.MarketRegime = regime

	return ctx
}

// getAverageWinRate 从统计表获取信号类型的平均胜率
func (a *OpportunityAggregator) getAverageWinRate(signals []*models.Signal) float64 {
	if a.statsRepo == nil {
		return 0
	}

	totalWinRate := 0.0
	count := 0
	for _, sig := range signals {
		symbolID := sig.SymbolID
		stat, err := a.statsRepo.GetBySignal(sig.SignalType, sig.Direction, sig.Period, &symbolID)
		if err != nil || stat == nil {
			// 尝试全局统计
			stat, err = a.statsRepo.GetBySignal(sig.SignalType, sig.Direction, sig.Period, nil)
		}
		if err == nil && stat != nil && stat.TotalTrades >= 5 {
			totalWinRate += stat.WinRate
			count++
		}
	}

	if count == 0 {
		return 0
	}
	return totalWinRate / float64(count)
}

// extractVolumeRatio 从信号数据中提取成交量比率
func (a *OpportunityAggregator) extractVolumeRatio(signals []*models.Signal) float64 {
	for _, sig := range signals {
		if sig.SignalData != nil {
			// 优先查找 volume_ratio
			if vr, ok := (*sig.SignalData)["volume_ratio"]; ok {
				if f, ok := vr.(float64); ok {
					return f
				}
			}
			// fallback: 兼容 volume_amplification
			if vr, ok := (*sig.SignalData)["volume_amplification"]; ok {
				if f, ok := vr.(float64); ok {
					return f
				}
			}
		}
	}
	return 0
}

// isTimeGapTooLarge 检查新信号与已有机会的时间跨度是否过大
// 同一个交易机会的信号应该集中在一段时间窗口内，跨度过大说明不属于同一行情
func (a *OpportunityAggregator) isTimeGapTooLarge(opp *models.TradingOpportunity, newSignals []*models.Signal) bool {
	if opp.FirstSignalAt == nil || len(newSignals) == 0 {
		return false
	}

	// 取新信号中最早的 klineTime
	var newTime *time.Time
	for _, sig := range newSignals {
		if sig.KlineTime != nil {
			if newTime == nil || sig.KlineTime.Before(*newTime) {
				newTime = sig.KlineTime
			}
		}
	}
	if newTime == nil {
		return false
	}

	// 计算允许的最大时间窗口（基于机会的周期，允许最多 3 根 K 线的跨度）
	maxWindow := klineCountToDuration(opp.Period, 3)
	gap := newTime.Sub(*opp.FirstSignalAt)

	return gap > maxWindow
}

// createOpportunity 创建新交易机会
func (a *OpportunityAggregator) createOpportunity(signals []*models.Signal, ctx *ScoringContext) {
	result := a.scorer.ScoreOpportunity(signals, ctx)

	// 过滤弱信号：评分低于阈值且信号数不足2个的不创建机会
	if result.TotalScore < a.minScoreToCreate && len(signals) < 2 {
		a.logger.Debug("跳过弱信号",
			zap.String("symbol", signals[0].SymbolCode),
			zap.Int("score", result.TotalScore),
			zap.Int("signal_count", len(signals)),
		)
		return
	}

	// 计算建议入场价、止损、止盈
	entry := signals[0].Price
	var stopLoss, takeProfit *float64
	for _, sig := range signals {
		// 优先保留已有的有效值，nil 不覆盖
		if stopLoss == nil && sig.StopLossPrice != nil {
			stopLoss = sig.StopLossPrice
		}
		if takeProfit == nil && sig.TargetPrice != nil {
			takeProfit = sig.TargetPrice
		}
	}

	// 共识方向（存具体信号类型，如 engulfing_bullish:long）
	directions := make([]string, 0, len(signals))
	for _, sig := range signals {
		directions = append(directions, sig.SignalType+":"+sig.Direction)
	}

	// 评分明细 JSON
	scoreDetails, _ := json.Marshal(result.Breakdown)
	scoreDetailsJSONB := models.JSONB(result.Breakdown)

	var firstSignalAt, lastSignalAt *time.Time
	for _, sig := range signals {
		if sig.KlineTime != nil {
			t := *sig.KlineTime
			if firstSignalAt == nil || t.Before(*firstSignalAt) {
				firstSignalAt = &t
			}
			if lastSignalAt == nil || t.After(*lastSignalAt) {
				lastSignalAt = &t
			}
		}
	}

	opp := &models.TradingOpportunity{
		SymbolID:             signals[0].SymbolID,
		SymbolCode:           signals[0].SymbolCode,
		Direction:            signals[0].Direction,
		Score:                result.TotalScore,
		ScoreDetails:         &scoreDetailsJSONB,
		SignalCount:          len(signals),
		ConfluenceDirections: directions,
		SuggestedEntry:       &entry,
		SuggestedStopLoss:    stopLoss,
		SuggestedTakeProfit:  takeProfit,
		Status:               models.OpportunityStatusActive,
		Period:               signals[0].Period,
		FirstSignalAt:        firstSignalAt,
		LastSignalAt:         lastSignalAt,
	}

	if err := a.oppRepo.Create(opp); err != nil {
		a.logger.Error("创建交易机会失败", zap.Error(err))
		return
	}

	// 过期同标的其他方向的机会
	if err := a.oppRepo.ExpireBySymbol(opp.SymbolID, opp.ID); err != nil {
		a.logger.Error("过期旧交易机会失败", zap.Error(err))
	}

	a.logger.Info("创建交易机会",
		zap.Int("id", opp.ID),
		zap.String("symbol", opp.SymbolCode),
		zap.String("direction", opp.Direction),
		zap.Int("score", opp.Score),
		zap.Int("signal_count", opp.SignalCount),
	)

	// 发送飞书通知（评分低于阈值时不通知）
	a.notifyIfNeeded(opp)

	_ = scoreDetails // avoid unused warning

	// 触发回调（自动交易等）
	a.invokeHandlers(opp)
}

// notifyIfNeeded 发送通知（如果评分达到阈值）
func (a *OpportunityAggregator) notifyIfNeeded(opp *models.TradingOpportunity) {
	if a.notifier != nil && opp.Score >= a.minScoreToNotify {
		if err := a.notifier.SendOpportunity(opp); err != nil {
			a.logger.Error("发送交易机会通知失败",
				zap.Int("id", opp.ID),
				zap.String("symbol", opp.SymbolCode),
				zap.Int("score", opp.Score),
				zap.Error(err))
		}
	}
}

// updateOpportunity 更新已有交易机会
func (a *OpportunityAggregator) updateOpportunity(opp *models.TradingOpportunity, newSignals []*models.Signal, ctx *ScoringContext) {
	// 增加信号计数
	opp.SignalCount += len(newSignals)

	// 更新共识方向
	for _, sig := range newSignals {
		opp.ConfluenceDirections = append(opp.ConfluenceDirections, sig.SignalType+":"+sig.Direction)
	}

	// 重新评分（opp.Signals 不从 DB 加载，只用 newSignals 评分）
	result := a.scorer.ScoreOpportunity(newSignals, ctx)
	opp.Score = result.TotalScore

	// 更新最后信号时间
	for _, sig := range newSignals {
		if sig.KlineTime != nil {
			if opp.LastSignalAt == nil || sig.KlineTime.After(*opp.LastSignalAt) {
				opp.LastSignalAt = sig.KlineTime
			}
		}
	}

	// 更新建议价格：入场价跟随最新信号，止损/止盈同步更新
	for _, sig := range newSignals {
		if sig.StopLossPrice != nil {
			opp.SuggestedStopLoss = sig.StopLossPrice
		}
		if sig.TargetPrice != nil {
			opp.SuggestedTakeProfit = sig.TargetPrice
		}
		opp.SuggestedEntry = &sig.Price
	}

	// 更新评分明细
	scoreDetailsJSONB := models.JSONB(result.Breakdown)
	opp.ScoreDetails = &scoreDetailsJSONB

	if err := a.oppRepo.Update(opp); err != nil {
		a.logger.Error("更新交易机会失败", zap.Error(err))
		return
	}

	a.logger.Info("更新交易机会",
		zap.Int("id", opp.ID),
		zap.String("symbol", opp.SymbolCode),
		zap.Int("score", opp.Score),
		zap.Int("signal_count", opp.SignalCount),
	)

	// 触发回调（评分更新后可能触发自动交易）
	a.invokeHandlers(opp)
}

// klineCountToDuration K线数量转时间持续时间
func klineCountToDuration(period string, klines int) time.Duration {
	switch period {
	case "1m":
		return time.Duration(klines) * time.Minute
	case "5m":
		return time.Duration(klines) * 5 * time.Minute
	case "15m":
		return time.Duration(klines) * 15 * time.Minute
	case "1h":
		return time.Duration(klines) * time.Hour
	case "4h":
		return time.Duration(klines) * 4 * time.Hour
	case "1d":
		return time.Duration(klines) * 24 * time.Hour
	default:
		return time.Duration(klines) * time.Hour
	}
}

// Helper to convert ScoreResult.Breakdown to JSONB
func scoreResultToJSONB(result *models.ScoreResult) *models.JSONB {
	if result == nil {
		return nil
	}
	b, _ := json.Marshal(result.Breakdown)
	var jsonb models.JSONB
	json.Unmarshal(b, &jsonb)
	return &jsonb
}

// computeRegimeMatchScore 基于4h趋势计算市场状态匹配度
func (a *OpportunityAggregator) computeRegimeMatchScore(signals []*models.Signal, symbolID int) (string, float64) {
	if a.symbolRepo == nil || len(signals) == 0 {
		return "unknown", 0.5
	}

	symbol, err := a.symbolRepo.GetByID(symbolID)
	if err != nil || symbol == nil || symbol.Trend4h == nil || *symbol.Trend4h == "" {
		return "unknown", 0.5
	}

	direction := signals[0].Direction
	trend4h := *symbol.Trend4h
	matchScore := calcTrendDirectionScore(trend4h, direction, signals)
	return trend4h, matchScore
}

// calcTrendDirectionScore 根据4h趋势、方向和信号类型计算匹配度 (0.0-1.0)
func calcTrendDirectionScore(trend, direction string, signals []*models.Signal) float64 {
	if trend == models.TrendTypeSideways {
		return 0.5
	}

	trendFollowing := 0
	contrarian := 0

	for _, sig := range signals {
		switch sig.SignalType {
		// 顺势信号
		case models.SignalTypeMACD, models.SignalTypeBoxBreakout,
			models.SignalTypePriceSurgeUp, models.SignalTypeResistanceBreak,
			models.SignalTypeMomentumBullish, models.SignalTypeEngulfingBullish,
			models.SignalTypeMorningStar,
			models.SignalTypeBoxBreakdown, models.SignalTypePriceSurgeDown,
			models.SignalTypeSupportBreak, models.SignalTypeMomentumBearish,
			models.SignalTypeEngulfingBearish, models.SignalTypeEveningStar:
			trendFollowing++

		// 逆势信号
		case models.SignalTypeVolumeSurge:
			contrarian++

		// 引线信号：按趋势方向判断
		case models.SignalTypeLowerWickReversal:
			if trend == models.TrendTypeBearish {
				contrarian++
			} else {
				trendFollowing++
			}
		case models.SignalTypeUpperWickReversal:
			if trend == models.TrendTypeBullish {
				contrarian++
			} else {
				trendFollowing++
			}
		default:
			trendFollowing++
		}
	}

	bullish := trend == models.TrendTypeBullish
	isLong := direction == models.DirectionLong

	// 顺势：趋势方向与交易方向一致
	if (bullish && isLong) || (!bullish && !isLong) {
		return 0.85
	}

	// 逆势：检查是否有足够的逆势信号支撑
	total := len(signals)
	if total > 0 && float64(contrarian)/float64(total) >= 0.5 {
		return 0.6
	}

	// 逆势且无逆势信号支撑
	return 0.2
}
