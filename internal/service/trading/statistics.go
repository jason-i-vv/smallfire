package trading

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/smallfire/starfire/internal/config"
	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/internal/repository"
)

// StatisticsService 交易统计分析服务
type StatisticsService struct {
	trackRepo  repository.TradeTrackRepo
	signalRepo repository.SignalRepo
	oppRepo    repository.OpportunityRepo
	symbolRepo repository.SymbolRepo
	config     *config.TradingConfig
}

// NewStatisticsService 创建统计分析服务实例
func NewStatisticsService(trackRepo repository.TradeTrackRepo, signalRepo repository.SignalRepo, oppRepo repository.OpportunityRepo, symbolRepo repository.SymbolRepo, cfg *config.TradingConfig) *StatisticsService {
	return &StatisticsService{
		trackRepo:  trackRepo,
		signalRepo: signalRepo,
		oppRepo:    oppRepo,
		symbolRepo: symbolRepo,
		config:     cfg,
	}
}

// TradeStatistics 交易统计
type TradeStatistics struct {
	// 基本统计
	TotalTrades int     `json:"total_trades"`
	WinTrades   int     `json:"win_trades"`
	LossTrades  int     `json:"loss_trades"`
	WinRate     float64 `json:"win_rate"`

	// 盈亏统计
	TotalPnL     float64 `json:"total_pnl"`
	AvgWin       float64 `json:"avg_win"`
	AvgLoss      float64 `json:"avg_loss"`
	ProfitFactor float64 `json:"profit_factor"`
	Expectancy   float64 `json:"expectancy"` // 期望值

	// 风控统计
	MaxDrawdown        float64 `json:"max_drawdown"`
	MaxDrawdownPct     float64 `json:"max_drawdown_pct"`
	MaxConsecutiveWin  int     `json:"max_consecutive_win"`
	MaxConsecutiveLoss int     `json:"max_consecutive_loss"`

	// 绩效指标
	SharpeRatio     float64 `json:"sharpe_ratio"`
	CalmarRatio     float64 `json:"calmar_ratio"`
	AvgHoldingHours float64 `json:"avg_holding_hours"`

	// 账户信息
	InitialCapital float64 `json:"initial_capital"`
	CurrentCapital float64 `json:"current_capital"`
	TotalReturn    float64 `json:"total_return"`
}

// GetStatistics 获取统计数据
func (s *StatisticsService) GetStatistics(startDate, endDate *time.Time, tradeSource string) (*TradeStatistics, error) {
	stats := &TradeStatistics{InitialCapital: s.config.InitialCapital}

	// 基本统计（SQL聚合）
	basic, err := s.trackRepo.GetBasicStatsSQL(startDate, endDate, tradeSource)
	if err != nil {
		return nil, fmt.Errorf("获取基本统计失败: %w", err)
	}
	stats.TotalTrades = basic.TotalTrades
	stats.WinTrades = basic.WinTrades
	stats.LossTrades = basic.LossTrades
	stats.TotalPnL = basic.TotalPnL
	stats.AvgHoldingHours = basic.AvgHoldingHrs
	stats.CurrentCapital = s.config.InitialCapital + basic.TotalPnL
	stats.TotalReturn = basic.TotalPnL / s.config.InitialCapital
	if basic.WinTrades > 0 {
		stats.AvgWin = basic.TotalWin / float64(basic.WinTrades)
	}
	if basic.LossTrades > 0 {
		stats.AvgLoss = basic.TotalLoss / float64(basic.LossTrades)
	}
	if basic.TotalTrades > 0 {
		stats.WinRate = float64(basic.WinTrades) / float64(basic.TotalTrades)
	}
	if basic.TotalLoss > 0 {
		stats.ProfitFactor = basic.TotalWin / basic.TotalLoss
	}
	stats.Expectancy = stats.WinRate*stats.AvgWin - (1-stats.WinRate)*stats.AvgLoss

	if basic.TotalTrades == 0 {
		return stats, nil
	}

	// 轻量级数据用于复杂计算（回撤、连胜、夏普）
	lightData, err := s.trackRepo.GetLightTrackDataSQL(startDate, endDate, tradeSource)
	if err != nil {
		return nil, fmt.Errorf("获取交易数据失败: %w", err)
	}

	var cumulativePnL, peakCapital, maxDrawdown float64
	consecutiveWin, consecutiveLoss := 0, 0
	var returns []float64

	for _, d := range lightData {
		pnl := d.PnL
		cumulativePnL += pnl
		currentCapital := s.config.InitialCapital + cumulativePnL
		if currentCapital > peakCapital {
			peakCapital = currentCapital
		}
		if dd := peakCapital - currentCapital; dd > maxDrawdown {
			maxDrawdown = dd
		}
		if pnl > 0 {
			consecutiveWin++
			consecutiveLoss = 0
			if consecutiveWin > stats.MaxConsecutiveWin {
				stats.MaxConsecutiveWin = consecutiveWin
			}
		} else {
			consecutiveLoss++
			consecutiveWin = 0
			if consecutiveLoss > stats.MaxConsecutiveLoss {
				stats.MaxConsecutiveLoss = consecutiveLoss
			}
		}
		if d.PositionValue != nil && *d.PositionValue > 0 {
			returns = append(returns, pnl / *d.PositionValue)
		}
	}
	stats.MaxDrawdown = maxDrawdown
	if peakCapital > 0 {
		stats.MaxDrawdownPct = maxDrawdown / peakCapital
	}
	if len(returns) >= 2 {
		meanReturn := 0.0
		for _, r := range returns {
			meanReturn += r
		}
		meanReturn /= float64(len(returns))
		variance := 0.0
		for _, r := range returns {
			diff := r - meanReturn
			variance += diff * diff
		}
		variance /= float64(len(returns) - 1)
		if stdDev := math.Sqrt(variance); stdDev > 0 {
			annualFactor := math.Sqrt(365*24 / max(stats.AvgHoldingHours, 1))
			stats.SharpeRatio = (meanReturn / stdDev) * annualFactor
		}
	}
	if stats.MaxDrawdownPct > 0 {
		stats.CalmarRatio = stats.TotalReturn / stats.MaxDrawdownPct
	}
	return stats, nil
}

func (s *StatisticsService) calculateStatistics(tracks []*models.TradeTrack) (*TradeStatistics, error) {
	stats := &TradeStatistics{
		InitialCapital: s.config.InitialCapital,
	}

	if len(tracks) == 0 {
		stats.CurrentCapital = s.config.InitialCapital
		return stats, nil
	}

	stats.TotalTrades = len(tracks)

	var totalWin, totalLoss float64
	var cumulativePnL float64
	var peakCapital float64
	var maxDrawdown float64

	consecutiveWin, consecutiveLoss := 0, 0
	stats.MaxConsecutiveWin = 0
	stats.MaxConsecutiveLoss = 0

	var totalHoldingHours float64

	for _, track := range tracks {
		if track.PnL == nil {
			continue
		}

		pnl := *track.PnL
		if pnl > 0 {
			stats.WinTrades++
			totalWin += pnl
			consecutiveWin++
			consecutiveLoss = 0
			if consecutiveWin > stats.MaxConsecutiveWin {
				stats.MaxConsecutiveWin = consecutiveWin
			}
		} else {
			stats.LossTrades++
			totalLoss += math.Abs(pnl)
			consecutiveLoss++
			consecutiveWin = 0
			if consecutiveLoss > stats.MaxConsecutiveLoss {
				stats.MaxConsecutiveLoss = consecutiveLoss
			}
		}

		cumulativePnL += pnl
		currentCapital := s.config.InitialCapital + cumulativePnL

		// 计算最大回撤
		if currentCapital > peakCapital {
			peakCapital = currentCapital
		}
		drawdown := peakCapital - currentCapital
		if drawdown > maxDrawdown {
			maxDrawdown = drawdown
		}

		// 持仓时间
		if track.EntryTime != nil && track.ExitTime != nil {
			hours := track.ExitTime.Sub(*track.EntryTime).Hours()
			totalHoldingHours += hours
		}
	}

	// 计算统计指标
	stats.TotalPnL = cumulativePnL
	stats.CurrentCapital = s.config.InitialCapital + cumulativePnL
	stats.TotalReturn = cumulativePnL / s.config.InitialCapital

	if stats.WinTrades > 0 {
		stats.AvgWin = totalWin / float64(stats.WinTrades)
	}
	if stats.LossTrades > 0 {
		stats.AvgLoss = totalLoss / float64(stats.LossTrades)
	}
	if stats.LossTrades > 0 {
		stats.ProfitFactor = totalWin / totalLoss
	}
	if stats.TotalTrades > 0 {
		stats.WinRate = float64(stats.WinTrades) / float64(stats.TotalTrades)
	}

	// 期望值 = 胜率 * 平均盈利 - 败率 * 平均亏损
	stats.Expectancy = stats.WinRate*stats.AvgWin - (1-stats.WinRate)*stats.AvgLoss

	// 最大回撤百分比
	stats.MaxDrawdown = maxDrawdown
	if peakCapital > 0 {
		stats.MaxDrawdownPct = maxDrawdown / peakCapital
	}

	// 平均持仓时间
	if stats.TotalTrades > 0 {
		stats.AvgHoldingHours = totalHoldingHours / float64(stats.TotalTrades)
	}

	// 收集每笔收益率用于夏普比率计算
	var returns []float64
	for _, track := range tracks {
		if track.PnL != nil && track.PositionValue != nil && *track.PositionValue > 0 {
			returns = append(returns, *track.PnL / *track.PositionValue)
		}
	}

	// 计算夏普比率（年化，假设无风险利率 0）
	if len(returns) >= 2 {
		meanReturn := 0.0
		for _, r := range returns {
			meanReturn += r
		}
		meanReturn /= float64(len(returns))

		variance := 0.0
		for _, r := range returns {
			diff := r - meanReturn
			variance += diff * diff
		}
		variance /= float64(len(returns) - 1)
		stdDev := math.Sqrt(variance)

		if stdDev > 0 {
			// 年化因子: 假设平均持仓时间代表交易频率
			annualFactor := math.Sqrt(365 * 24 / max(stats.AvgHoldingHours, 1))
			stats.SharpeRatio = (meanReturn / stdDev) * annualFactor
		}
	}

	// 计算卡玛比率
	if stats.MaxDrawdownPct > 0 {
		stats.CalmarRatio = stats.TotalReturn / stats.MaxDrawdownPct
	}

	return stats, nil
}

// SignalAnalysis 信号分析统计
type SignalAnalysis struct {
	SignalType  string  `json:"signal_type"`
	SourceType  string  `json:"source_type"`
	TotalTrades int     `json:"total_trades"`
	WinTrades   int     `json:"win_trades"`
	WinRate     float64 `json:"win_rate"`
	TotalPnL    float64 `json:"total_pnl"`
}

// EquityCurvePoint 权益曲线点
type EquityCurvePoint struct {
	Time   int64   `json:"time"`    // Unix timestamp (seconds)
	Equity float64 `json:"equity"`
}

// SymbolAnalysis 按标的分析
type SymbolAnalysis struct {
	SymbolID    int     `json:"symbol_id"`
	SymbolCode  string  `json:"symbol_code"`
	TotalTrades int     `json:"total_trades"`
	WinTrades   int     `json:"win_trades"`
	WinRate     float64 `json:"win_rate"`
	TotalPnL    float64 `json:"total_pnl"`
	AvgPnL      float64 `json:"avg_pnl"`
}

// DirectionAnalysis 按方向分析
type DirectionAnalysis struct {
	Direction       string  `json:"direction"`
	TotalTrades     int     `json:"total_trades"`
	WinTrades       int     `json:"win_trades"`
	WinRate         float64 `json:"win_rate"`
	TotalPnL        float64 `json:"total_pnl"`
	AvgPnL          float64 `json:"avg_pnl"`
	AvgHoldingHours float64 `json:"avg_holding_hours"`
}

// ExitReasonAnalysis 按出场原因分析
type ExitReasonAnalysis struct {
	ExitReason  string  `json:"exit_reason"`
	TotalTrades int     `json:"total_trades"`
	WinTrades   int     `json:"win_trades"`
	WinRate     float64 `json:"win_rate"`
	TotalPnL    float64 `json:"total_pnl"`
}

// PeriodPnL 按时间周期的盈亏
type PeriodPnL struct {
	PeriodStart int64   `json:"period_start"` // Unix seconds
	PnL         float64 `json:"pnl"`
	TradeCount  int     `json:"trade_count"`
}

// PnLDistribution 盈亏分布
type PnLDistribution struct {
	Buckets []PnLBucket `json:"buckets"`
}

// PnLBucket 盈亏分布桶
type PnLBucket struct {
	RangeStart float64 `json:"range_start"`
	RangeEnd   float64 `json:"range_end"`
	Count      int     `json:"count"`
	IsWin      bool    `json:"is_win"`
}

// GetSignalAnalysis 按信号类型分析
func (s *StatisticsService) GetSignalAnalysis(tradeSource string) (map[string]*SignalAnalysis, error) {
	stats, err := s.trackRepo.GetSignalStatsSQL(nil, nil, tradeSource)
	if err != nil {
		return nil, fmt.Errorf("获取信号统计失败: %w", err)
	}
	analysis := make(map[string]*SignalAnalysis)
	for _, st := range stats {
		key := st.SignalType
		if _, ok := analysis[key]; !ok {
			analysis[key] = &SignalAnalysis{SignalType: st.SignalType, SourceType: st.SourceType}
		}
		a := analysis[key]
		a.TotalTrades += st.TotalTrades
		a.WinTrades += st.WinTrades
		a.TotalPnL += st.TotalPnL
	}
	for _, a := range analysis {
		if a.TotalTrades > 0 {
			a.WinRate = float64(a.WinTrades) / float64(a.TotalTrades)
		}
	}
	return analysis, nil
}

func (s *StatisticsService) getSignalType(track *models.TradeTrack) string {
	if s.signalRepo == nil || track.SignalID == nil {
		return "unknown"
	}
	signal, err := s.signalRepo.GetByID(*track.SignalID)
	if err != nil || signal == nil {
		return "unknown"
	}
	return signal.SourceType
}

// GetEquityCurve 获取权益曲线数据
func (s *StatisticsService) GetEquityCurve(startDate, endDate *time.Time, tradeSource string) ([]EquityCurvePoint, error) {
	sqlResults, err := s.trackRepo.GetEquityCurveSQL(startDate, endDate, tradeSource)
	if err != nil {
		return nil, fmt.Errorf("获取权益曲线失败: %w", err)
	}
	if len(sqlResults) == 0 {
		return nil, nil
	}

	// 按天聚合的 daily pnl → 累计 equity
	points := make([]EquityCurvePoint, 0, len(sqlResults)+1)
	points = append(points, EquityCurvePoint{Time: sqlResults[0].Time - 86400, Equity: s.config.InitialCapital})
	cumPnL := 0.0
	for _, r := range sqlResults {
		cumPnL += r.CumPnL // CumPnL now holds day_pnl
		points = append(points, EquityCurvePoint{Time: r.Time, Equity: s.config.InitialCapital + cumPnL})
	}
	return points, nil
}

// GetSymbolAnalysis 按标的统计
func (s *StatisticsService) GetSymbolAnalysis(startDate, endDate *time.Time, tradeSource string) ([]SymbolAnalysis, error) {
	stats, err := s.trackRepo.GetSymbolStatsSQL(startDate, endDate, tradeSource)
	if err != nil {
		return nil, fmt.Errorf("获取标的统计失败: %w", err)
	}
	result := make([]SymbolAnalysis, 0, len(stats))
	for _, st := range stats {
		g := SymbolAnalysis{SymbolID: st.SymbolID, TotalTrades: st.TotalTrades, WinTrades: st.WinTrades, TotalPnL: st.TotalPnL}
		if s.symbolRepo != nil {
			sym, err := s.symbolRepo.GetByID(st.SymbolID)
			if err == nil && sym != nil {
				g.SymbolCode = sym.SymbolCode
			}
		}
		if g.TotalTrades > 0 {
			g.WinRate = float64(g.WinTrades) / float64(g.TotalTrades)
			g.AvgPnL = g.TotalPnL / float64(g.TotalTrades)
		}
		result = append(result, g)
	}
	return result, nil
}

// GetDirectionAnalysis 按方向统计
func (s *StatisticsService) GetDirectionAnalysis(startDate, endDate *time.Time, tradeSource string) (map[string]*DirectionAnalysis, error) {
	stats, err := s.trackRepo.GetDirectionStatsSQL(startDate, endDate, tradeSource)
	if err != nil {
		return nil, fmt.Errorf("获取方向统计失败: %w", err)
	}
	analysis := map[string]*DirectionAnalysis{
		"long":  {Direction: "long"},
		"short": {Direction: "short"},
	}
	for _, st := range stats {
		a := &DirectionAnalysis{
			Direction:      st.Direction,
			TotalTrades:    st.TotalTrades,
			WinTrades:      st.WinTrades,
			TotalPnL:       st.TotalPnL,
			AvgHoldingHours: st.AvgHoldingHrs,
		}
		if a.TotalTrades > 0 {
			a.WinRate = float64(a.WinTrades) / float64(a.TotalTrades)
			a.AvgPnL = a.TotalPnL / float64(a.TotalTrades)
		}
		analysis[st.Direction] = a
	}
	return analysis, nil
}

// GetExitReasonAnalysis 按出场原因统计
func (s *StatisticsService) GetExitReasonAnalysis(startDate, endDate *time.Time, tradeSource string) ([]ExitReasonAnalysis, error) {
	stats, err := s.trackRepo.GetExitReasonStatsSQL(startDate, endDate, tradeSource)
	if err != nil {
		return nil, fmt.Errorf("获取出场原因统计失败: %w", err)
	}
	result := make([]ExitReasonAnalysis, 0, len(stats))
	for _, st := range stats {
		a := ExitReasonAnalysis{ExitReason: st.ExitReason, TotalTrades: st.TotalTrades, WinTrades: st.WinTrades, TotalPnL: st.TotalPnL}
		if a.TotalTrades > 0 {
			a.WinRate = float64(a.WinTrades) / float64(a.TotalTrades)
		}
		result = append(result, a)
	}
	return result, nil
}

// GetPeriodPnL 按时间周期统计盈亏
func (s *StatisticsService) GetPeriodPnL(startDate, endDate *time.Time, period string, tradeSource string) ([]PeriodPnL, error) {
	stats, err := s.trackRepo.GetPeriodPnLSQL(startDate, endDate, period, tradeSource)
	if err != nil {
		return nil, fmt.Errorf("获取周期盈亏失败: %w", err)
	}
	result := make([]PeriodPnL, 0, len(stats))
	for _, st := range stats {
		result = append(result, PeriodPnL{PeriodStart: st.PeriodStart.Unix(), PnL: st.PnL, TradeCount: st.TradeCount})
	}
	return result, nil
}

// GetPnLDistribution 获取盈亏分布
func (s *StatisticsService) GetPnLDistribution(startDate, endDate *time.Time, tradeSource string) (*PnLDistribution, error) {
	pnls, err := s.trackRepo.GetPnLValuesSQL(startDate, endDate, tradeSource)
	if err != nil {
		return nil, fmt.Errorf("获取PnL分布失败: %w", err)
	}
	if len(pnls) == 0 {
		return &PnLDistribution{Buckets: []PnLBucket{}}, nil
	}

	minPnL, maxPnL := pnls[0], pnls[0]
	for _, p := range pnls {
		if p < minPnL { minPnL = p }
		if p > maxPnL { maxPnL = p }
	}

	bucketCount := 20
	rangeSize := (maxPnL - minPnL) / float64(bucketCount)
	if rangeSize == 0 { rangeSize = 1 }
	buckets := make([]PnLBucket, bucketCount)
	for i := 0; i < bucketCount; i++ {
		buckets[i] = PnLBucket{
			RangeStart: minPnL + float64(i)*rangeSize,
			RangeEnd:   minPnL + float64(i+1)*rangeSize,
			IsWin:      (minPnL + float64(i)*rangeSize) >= 0,
		}
	}
	for _, p := range pnls {
		idx := int((p - minPnL) / rangeSize)
		if idx >= bucketCount { idx = bucketCount - 1 }
		if idx < 0 { idx = 0 }
		buckets[idx].Count++
	}
	return &PnLDistribution{Buckets: buckets}, nil
}

// GetDetailedSignalAnalysis 按具体信号类型分析
func (s *StatisticsService) GetDetailedSignalAnalysis(startDate, endDate *time.Time, tradeSource string) ([]SignalAnalysis, error) {
	stats, err := s.trackRepo.GetSignalStatsSQL(startDate, endDate, tradeSource)
	if err != nil {
		return nil, fmt.Errorf("获取信号统计失败: %w", err)
	}
	analysis := make(map[string]*SignalAnalysis)
	for _, st := range stats {
		key := st.SignalType
		if _, ok := analysis[key]; !ok {
			analysis[key] = &SignalAnalysis{SignalType: st.SignalType, SourceType: st.SourceType}
		}
		a := analysis[key]
		a.TotalTrades += st.TotalTrades
		a.WinTrades += st.WinTrades
		a.TotalPnL += st.TotalPnL
	}
	result := make([]SignalAnalysis, 0, len(analysis))
	for _, a := range analysis {
		if a.TotalTrades > 0 {
			a.WinRate = float64(a.WinTrades) / float64(a.TotalTrades)
		}
		result = append(result, *a)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].TotalPnL > result[j].TotalPnL
	})
	return result, nil
}

// getFullSignalInfo 获取信号的完整信息（signal_type + source_type）
// signalTypeToSourceType 映射各信号类型对应的策略来源
var signalTypeToSourceType = map[string]string{
	// candlestick
	"engulfing_bullish": "candlestick",
	"engulfing_bearish": "candlestick",
	"momentum_bullish":  "candlestick",
	"momentum_bearish":  "candlestick",
	"morning_star":      "candlestick",
	"evening_star":      "candlestick",
	// wick
	"upper_wick_reversal": "wick",
	"lower_wick_reversal": "wick",
	"fake_breakout_upper": "wick",
	"fake_breakout_lower": "wick",
	// key_level
	"resistance_break": "key_level",
	"support_break":    "key_level",
	// volume
	"volume_surge":      "volume",
	"volume_price_rise":  "volume",
	"volume_price_fall":  "volume",
	"price_surge_up":    "volume",
	"price_surge_down":  "volume",
	// trend
	"trend_retracement": "trend",
}

// ScoreAnalysis 按评分区间分析
type ScoreAnalysis struct {
	ScoreRange      string  `json:"score_range"`       // 评分区间，如 "80-100"
	TotalTrades     int     `json:"total_trades"`      // 交易次数
	WinTrades       int     `json:"win_trades"`        // 盈利次数
	WinRate         float64 `json:"win_rate"`          // 胜率
	TotalPnL        float64 `json:"total_pnl"`         // 总盈亏
	AvgPnL          float64 `json:"avg_pnl"`           // 平均盈亏
	AvgHoldingHours float64 `json:"avg_holding_hours"` // 平均持仓时长
}

// StrategyAnalysis 按策略类型分析
type StrategyAnalysis struct {
	Strategy       string  `json:"strategy"`        // 策略名称，如 "箱体", "趋势"
	StrategyKey    string  `json:"strategy_key"`     // 策略标识，如 "box", "trend"
	TotalTrades    int     `json:"total_trades"`     // 交易次数
	WinTrades      int     `json:"win_trades"`       // 盈利次数
	WinRate        float64 `json:"win_rate"`         // 胜率
	TotalPnL       float64 `json:"total_pnl"`        // 总盈亏
	AvgPnL         float64 `json:"avg_pnl"`          // 平均盈亏
	AvgHoldingHours float64 `json:"avg_holding_hours"` // 平均持仓时长
}

// RegimeAnalysis 市场状态分析
type RegimeAnalysis struct {
	Regime         string  `json:"regime"`          // 市场状态: 顺势, 逆势, 震荡
	TotalTrades    int     `json:"total_trades"`     // 交易次数
	WinTrades      int     `json:"win_trades"`       // 盈利次数
	WinRate        float64 `json:"win_rate"`         // 胜率
	TotalPnL       float64 `json:"total_pnl"`        // 总盈亏
	AvgPnL         float64 `json:"avg_pnl"`          // 平均盈亏
	AvgHoldingHours float64 `json:"avg_holding_hours"` // 平均持仓时长
}

// StrategyRegimeAnalysis 策略 × 市场状态 交叉分析
type StrategyRegimeAnalysis struct {
	Strategy    string                     `json:"strategy"`
	StrategyKey string                     `json:"strategy_key"`
	Overall     StrategyRegimeStats          `json:"overall"`
	Regimes     map[string]RegimeStats     `json:"regimes"` // key: 顺势, 逆势, 震荡
}

// StrategyRegimeStats 策略在某市场状态下的统计
type StrategyRegimeStats struct {
	TotalTrades    int     `json:"total_trades"`
	WinTrades      int     `json:"win_trades"`
	WinRate        float64 `json:"win_rate"`
	TotalPnL       float64 `json:"total_pnl"`
	AvgPnL         float64 `json:"avg_pnl"`
}

// RegimeStats 市场状态统计数据
type RegimeStats struct {
	TotalTrades int     `json:"total_trades"`
	WinTrades   int     `json:"win_trades"`
	WinRate     float64 `json:"win_rate"`
	TotalPnL    float64 `json:"total_pnl"`
	AvgPnL      float64 `json:"avg_pnl"`
}

// ScoreRegimeAnalysis 评分维度 × 市场状态 交叉分析
type ScoreRegimeAnalysis struct {
	Dimension string                     `json:"dimension"` // 维度名称
	Weight    float64                    `json:"weight"`    // 权重
	Ranges    map[string]RegimeStats     `json:"ranges"`    // key: 评分区间
}

// regimeLabels 市场状态标签映射
var regimeLabels = map[string]string{
	"bullish":  "顺势",
	"bearish":  "逆势",
	"sideways": "震荡",
	"":         "震荡",
	"unknown":  "震荡",
}

// computeRegime 根据 trend4h 和 direction 计算市场状态
func computeRegime(trend4h, direction string) string {
	// 空趋势或未知趋势视为震荡
	if trend4h == "" || trend4h == "unknown" {
		return "震荡"
	}

	// 多头趋势 + 做多 = 顺势
	// 空头趋势 + 做空 = 顺势
	if (trend4h == "bullish" && direction == "long") ||
		(trend4h == "bearish" && direction == "short") {
		return "顺势"
	}

	// 多头趋势 + 做空 = 逆势
	// 空头趋势 + 做多 = 逆势
	if (trend4h == "bullish" && direction == "short") ||
		(trend4h == "bearish" && direction == "long") {
		return "逆势"
	}

	// 其他情况视为震荡
	return "震荡"
}

// GetScoreAnalysis 按评分区间统计胜率
func (s *StatisticsService) GetScoreAnalysis(startDate, endDate *time.Time, tradeSource string) ([]ScoreAnalysis, error) {
	stats, err := s.trackRepo.GetScoreStatsSQL(startDate, endDate, tradeSource)
	if err != nil {
		return nil, fmt.Errorf("获取评分统计失败: %w", err)
	}

	rangeOrder := map[string]int{"80-100": 0, "70-80": 1, "60-70": 2, "50-60": 3, "<50": 4}
	resultMap := make(map[string]*ScoreAnalysis)
	for _, st := range stats {
		a := &ScoreAnalysis{
			ScoreRange:      st.ScoreRange,
			TotalTrades:     st.TotalTrades,
			WinTrades:       st.WinTrades,
			TotalPnL:        st.TotalPnL,
			AvgHoldingHours: st.AvgHoldingHrs,
		}
		if a.TotalTrades > 0 {
			a.WinRate = float64(a.WinTrades) / float64(a.TotalTrades)
			a.AvgPnL = a.TotalPnL / float64(a.TotalTrades)
		}
		resultMap[st.ScoreRange] = a
	}

	// 确保所有区间都存在
	for _, name := range []string{"80-100", "70-80", "60-70", "50-60", "<50"} {
		if _, ok := resultMap[name]; !ok {
			resultMap[name] = &ScoreAnalysis{ScoreRange: name}
		}
	}

	result := make([]ScoreAnalysis, 0, 5)
	for _, name := range []string{"80-100", "70-80", "60-70", "50-60", "<50"} {
		result = append(result, *resultMap[name])
	}

	_ = rangeOrder
	return result, nil
}

// strategyLabels 策略标签映射
var strategyLabels = map[string]string{
	"box":         "箱体",
	"trend":       "趋势",
	"key_level":   "关键位",
	"volume":      "量价",
	"wick":        "引线",
	"candlestick": "K线",
		"macd":        "MACD",
	"unknown":     "未知",
}

// GetStrategyAnalysis 按策略类型统计盈亏
func (s *StatisticsService) GetStrategyAnalysis(startDate, endDate *time.Time, tradeSource string) ([]StrategyAnalysis, error) {
	stats, err := s.trackRepo.GetStrategyStatsSQL(startDate, endDate, tradeSource)
	if err != nil {
		return nil, fmt.Errorf("获取策略统计失败: %w", err)
	}
	result := make([]StrategyAnalysis, 0, len(stats))
	for _, st := range stats {
		a := StrategyAnalysis{
			Strategy:       strategyLabels[st.SourceType],
			StrategyKey:    st.SourceType,
			TotalTrades:    st.TotalTrades,
			WinTrades:      st.WinTrades,
			TotalPnL:       st.TotalPnL,
			AvgHoldingHours: st.AvgHoldingHrs,
		}
		if a.TotalTrades > 0 {
			a.WinRate = float64(a.WinTrades) / float64(a.TotalTrades)
			a.AvgPnL = a.TotalPnL / float64(a.TotalTrades)
		}
		result = append(result, a)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].TotalPnL > result[j].TotalPnL
	})
	return result, nil
}

// ScoreEquityCurvePoint 按评分的权益曲线数据点
type ScoreEquityCurvePoint struct {
	Time int64   `json:"time"` // Unix seconds
	PnL  float64 `json:"pnl"`  // 累计盈亏
}

// ScoreEquityCurves 按评分的权益曲线集合
type ScoreEquityCurves struct {
	Ranges []ScoreRangeCurve `json:"ranges"`
}

// ScoreRangeCurve 单个评分区间的曲线
type ScoreRangeCurve struct {
	ScoreRange string                  `json:"score_range"`
	Color      string                  `json:"color"`
	Data       []ScoreEquityCurvePoint `json:"data"`
}

// scoreRangeDefs 评分区间定义及颜色
var scoreRangeDefs = []struct {
	name  string
	min   int
	max   int
	color string
}{
	{"80-100", 80, 100, "#00C853"},  // 绿色
	{"70-80", 70, 79, "#4CAF50"},    // 浅绿
	{"60-70", 60, 69, "#FFC107"},    // 黄色
	{"50-60", 50, 59, "#FF9800"},    // 橙色
	{"<50", 0, 49, "#F44336"},       // 红色
}

// getScoreRange 获取评分所在区间名称
func getScoreRange(score int) string {
	for _, r := range scoreRangeDefs {
		if score >= r.min && score <= r.max {
			return r.name
		}
	}
	return "<50"
}

// GetScoreEquityCurves 按评分等级获取每日累计盈亏曲线
func (s *StatisticsService) GetScoreEquityCurves(startDate, endDate *time.Time, tradeSource string) (*ScoreEquityCurves, error) {
	sqlResults, err := s.trackRepo.GetScoreEquitySQL(startDate, endDate, tradeSource)
	if err != nil {
		return nil, fmt.Errorf("获取评分权益数据失败: %w", err)
	}

	// 按 score_range 分组，按天排序，计算累计
	rangeDailyPnL := make(map[string][]struct{ ts int64; pnl float64 })
	for _, r := range sqlResults {
		rangeDailyPnL[r.ScoreRange] = append(rangeDailyPnL[r.ScoreRange], struct{ ts int64; pnl float64 }{r.DayTs, r.DayPnL})
	}

	result := &ScoreEquityCurves{}
	for _, def := range scoreRangeDefs {
		curve := ScoreRangeCurve{ScoreRange: def.name, Color: def.color}
		items := rangeDailyPnL[def.name]
		cumulative := 0.0
		for _, item := range items {
			cumulative += item.pnl
			curve.Data = append(curve.Data, ScoreEquityCurvePoint{
				Time: item.ts,
				PnL:  math.Round(cumulative*100) / 100,
			})
		}
		result.Ranges = append(result.Ranges, curve)
	}
	return result, nil
}

// collectOpportunityIDs 从 tracks 中收集所有不重复的 opportunity_id
func collectOpportunityIDs(tracks []*models.TradeTrack) []int {
	seen := make(map[int]bool)
	var ids []int
	for _, t := range tracks {
		if t.OpportunityID != nil && !seen[*t.OpportunityID] {
			seen[*t.OpportunityID] = true
			ids = append(ids, *t.OpportunityID)
		}
	}
	return ids
}

// collectSignalIDs 从 tracks 中收集所有不重复的 signal_id
func collectSignalIDs(tracks []*models.TradeTrack) []int {
	seen := make(map[int]bool)
	var ids []int
	for _, t := range tracks {
		if t.SignalID != nil && !seen[*t.SignalID] {
			seen[*t.SignalID] = true
			ids = append(ids, *t.SignalID)
		}
	}
	return ids
}

// signalInfoContext 预加载的信号和机会信息，避免 N+1 查询
type signalInfoContext struct {
	signalMap     map[int]*repository.SignalBasicInfo // signalID -> info
	confluenceMap map[int][]string                    // opportunityID -> confluence_directions
}

// buildSignalInfoContext 预加载所有需要的信号和机会数据
func (s *StatisticsService) buildSignalInfoContext(tracks []*models.TradeTrack) *signalInfoContext {
	ctx := &signalInfoContext{
		signalMap:     make(map[int]*repository.SignalBasicInfo),
		confluenceMap: make(map[int][]string),
	}

	// 批量获取 signal 信息
	if s.signalRepo != nil {
		signalIDs := collectSignalIDs(tracks)
		if len(signalIDs) > 0 {
			if m, err := s.signalRepo.GetSignalInfoByIDs(signalIDs); err == nil {
				ctx.signalMap = m
			}
		}
	}

	// 批量获取 opportunity confluence_directions
	if s.oppRepo != nil {
		oppIDs := collectOpportunityIDs(tracks)
		if len(oppIDs) > 0 {
			if m, err := s.oppRepo.GetConfluenceByIDs(oppIDs); err == nil {
				ctx.confluenceMap = m
			}
		}
	}

	return ctx
}

// getFullSignalInfoFromContext 使用预加载数据获取信号信息
func (s *StatisticsService) getFullSignalInfoFromContext(track *models.TradeTrack, siCtx *signalInfoContext) (signalType, sourceType string) {
	// 优先通过 SignalID 获取
	if track.SignalID != nil {
		if info, ok := siCtx.signalMap[*track.SignalID]; ok {
			return info.SignalType, info.SourceType
		}
	}

	// SignalID 为空时，从 confluence_directions 推断
	if track.OpportunityID != nil {
		if directions, ok := siCtx.confluenceMap[*track.OpportunityID]; ok && len(directions) > 0 {
			first := directions[0]
			for i := 0; i < len(first); i++ {
				if first[i] == ':' {
					st := first[:i]
					src := signalTypeToSourceType[st]
					if src == "" {
						src = "unknown"
					}
					return st, src
				}
			}
			return first, "unknown"
		}
	}

	return "unknown", "unknown"
}

func (s *StatisticsService) getFullSignalInfo(track *models.TradeTrack) (signalType, sourceType string) {
	// 优先通过 SignalID 获取
	if track.SignalID != nil && s.signalRepo != nil {
		signal, err := s.signalRepo.GetByID(*track.SignalID)
		if err == nil && signal != nil {
			return signal.SignalType, signal.SourceType
		}
	}

	// SignalID 为空时，通过 OpportunityID 获取机会，再从 confluence_directions 推断信号类型
	if track.OpportunityID != nil && s.oppRepo != nil {
		opp, err := s.oppRepo.GetByID(*track.OpportunityID)
		if err == nil && opp != nil && opp.ConfluenceDirections != nil && len(opp.ConfluenceDirections) > 0 {
			// 取第一个信号类型（机会创建时的第一个信号）
			// confluence_directions 格式："signal_type:direction"
			first := opp.ConfluenceDirections[0]
			for i := 0; i < len(first); i++ {
				if first[i] == ':' {
					st := first[:i]
					src := signalTypeToSourceType[st]
					if src == "" {
						src = "unknown"
					}
					return st, src
				}
			}
			return first, "unknown"
		}
	}

	return "unknown", "unknown"
}

// GetRegimeAnalysis 获取市场状态统计分析
func (s *StatisticsService) GetRegimeAnalysis(startDate, endDate *time.Time, tradeSource string) ([]RegimeAnalysis, error) {
	// 使用 SQL 聚合查询优化性能
	stats, err := s.trackRepo.GetRegimeStatsSQL(startDate, endDate, tradeSource)
	if err != nil {
		return nil, fmt.Errorf("获取市场状态统计失败: %w", err)
	}

	// 构建结果，确保顺序
	regimeMap := make(map[string]RegimeAnalysis)
	for _, stat := range stats {
		a := RegimeAnalysis{
			Regime:         stat.Regime,
			TotalTrades:    stat.TotalTrades,
			WinTrades:      stat.WinTrades,
			TotalPnL:       stat.TotalPnL,
			AvgHoldingHours: stat.AvgHoldingHours,
		}
		if a.TotalTrades > 0 {
			a.WinRate = float64(a.WinTrades) / float64(a.TotalTrades)
			a.AvgPnL = a.TotalPnL / float64(a.TotalTrades)
		}
		regimeMap[stat.Regime] = a
	}

	// 确保三种市场状态都返回
	result := make([]RegimeAnalysis, 0, 3)
	for _, regime := range []string{"顺势", "逆势", "震荡"} {
		if a, ok := regimeMap[regime]; ok {
			result = append(result, a)
		} else {
			result = append(result, RegimeAnalysis{Regime: regime})
		}
	}

	return result, nil
}

// GetStrategyRegimeAnalysis 获取策略 × 市场状态 交叉分析
func (s *StatisticsService) GetStrategyRegimeAnalysis(startDate, endDate *time.Time, tradeSource string) ([]StrategyRegimeAnalysis, error) {
	// 使用 SQL 聚合查询优化性能
	stats, err := s.trackRepo.GetStrategyRegimeStatsSQL(startDate, endDate, tradeSource)
	if err != nil {
		return nil, fmt.Errorf("获取策略市场状态统计失败: %w", err)
	}

	// 按策略分组
	type rawStats struct {
		TotalTrades int
		WinTrades   int
		TotalPnL    float64
	}
	groups := make(map[string]map[string]*rawStats)

	for i := range stats {
		stat := &stats[i]
		if groups[stat.StrategyKey] == nil {
			groups[stat.StrategyKey] = make(map[string]*rawStats)
		}
		groups[stat.StrategyKey][stat.Regime] = &rawStats{
			TotalTrades: stat.TotalTrades,
			WinTrades:   stat.WinTrades,
			TotalPnL:    stat.TotalPnL,
		}
	}

	// 构建结果
	result := make([]StrategyRegimeAnalysis, 0, len(groups))
	for strategyKey, regimeGroups := range groups {
		item := StrategyRegimeAnalysis{
			Strategy:    strategyLabels[strategyKey],
			StrategyKey: strategyKey,
			Regimes:     make(map[string]RegimeStats),
		}

		// 计算总体统计
		var totalTrades, totalWins int
		var totalPnL float64
		for _, r := range regimeGroups {
			totalTrades += r.TotalTrades
			totalWins += r.WinTrades
			totalPnL += r.TotalPnL
		}
		if totalTrades > 0 {
			item.Overall = StrategyRegimeStats{
				TotalTrades: totalTrades,
				WinTrades:   totalWins,
				WinRate:     float64(totalWins) / float64(totalTrades),
				TotalPnL:    totalPnL,
				AvgPnL:      totalPnL / float64(totalTrades),
			}
		}

		// 各市场状态统计
		for _, regime := range []string{"顺势", "逆势", "震荡"} {
			if r := regimeGroups[regime]; r != nil && r.TotalTrades > 0 {
				item.Regimes[regime] = RegimeStats{
					TotalTrades: r.TotalTrades,
					WinTrades:   r.WinTrades,
					WinRate:     float64(r.WinTrades) / float64(r.TotalTrades),
					TotalPnL:    r.TotalPnL,
					AvgPnL:      r.TotalPnL / float64(r.TotalTrades),
				}
			}
		}

		result = append(result, item)
	}

	// 按总盈亏降序排列
	sort.Slice(result, func(i, j int) bool {
		return result[i].Overall.TotalPnL > result[j].Overall.TotalPnL
	})

	return result, nil
}

// GetScoreRegimeAnalysis 获取评分维度 × 市场状态 交叉分析
func (s *StatisticsService) GetScoreRegimeAnalysis(startDate, endDate *time.Time, tradeSource string) ([]ScoreRegimeAnalysis, error) {
	sqlResults, err := s.trackRepo.GetScoreRegimeSQL(startDate, endDate, tradeSource)
	if err != nil {
		return nil, fmt.Errorf("获取评分市场状态统计失败: %w", err)
	}

	// 构建 scoreRange -> regime -> stats 映射
	dataMap := make(map[string]map[string]*RegimeStats)
	for _, r := range sqlResults {
		if dataMap[r.ScoreRange] == nil {
			dataMap[r.ScoreRange] = make(map[string]*RegimeStats)
		}
		s := &RegimeStats{TotalTrades: r.TotalTrades, WinTrades: r.WinTrades, TotalPnL: r.TotalPnL}
		if s.TotalTrades > 0 {
			s.WinRate = float64(s.WinTrades) / float64(s.TotalTrades)
			s.AvgPnL = s.TotalPnL / float64(s.TotalTrades)
		}
		dataMap[r.ScoreRange][r.Regime] = s
	}

	dimensions := []struct {
		name   string
		weight float64
	}{
		{"市场状态匹配", 0.10},
		{"信号强度", 0.20},
		{"多策略共识", 0.25},
	}
	allRegimes := []string{"顺势", "逆势", "震荡"}

	result := make([]ScoreRegimeAnalysis, 0, len(dimensions))
	for _, dim := range dimensions {
		item := ScoreRegimeAnalysis{
			Dimension: dim.name,
			Weight:    dim.weight,
			Ranges:    make(map[string]RegimeStats),
		}
		for scoreRange, regimes := range dataMap {
			for _, regime := range allRegimes {
				if s, ok := regimes[regime]; ok && s.TotalTrades > 0 {
					key := regime
					if existing, ok := item.Ranges[key]; ok {
						// 合并
						existing.TotalTrades += s.TotalTrades
						existing.WinTrades += s.WinTrades
						existing.TotalPnL += s.TotalPnL
						if existing.TotalTrades > 0 {
							existing.WinRate = float64(existing.WinTrades) / float64(existing.TotalTrades)
							existing.AvgPnL = existing.TotalPnL / float64(existing.TotalTrades)
						}
						item.Ranges[key] = existing
					} else {
						item.Ranges[key] = *s
					}
				}
			}
			_ = scoreRange
		}
		result = append(result, item)
	}
	return result, nil
}

// ScoreGradeRegimeResult 评分区间 × 市场状态 交叉分析结果
type ScoreGradeRegimeResult struct {
	ScoreRange string                  `json:"score_range"` // 评分区间，如 "80-100", "60-80", "40-60", "0-40"
	Regimes    map[string]RegimeStats `json:"regimes"`     // key: 顺势/逆势/震荡
}

// GetScoreGradeRegimeAnalysis 获取评分区间 × 市场状态 交叉分析
func (s *StatisticsService) GetScoreGradeRegimeAnalysis(startDate, endDate *time.Time, tradeSource string) ([]ScoreGradeRegimeResult, error) {
	sqlResults, err := s.trackRepo.GetScoreRegimeSQL(startDate, endDate, tradeSource)
	if err != nil {
		return nil, fmt.Errorf("获取评分区间市场状态统计失败: %w", err)
	}

	// 构建 scoreRange -> regime -> stats 映射
	dataMap := make(map[string]map[string]*RegimeStats)
	for _, r := range sqlResults {
		if dataMap[r.ScoreRange] == nil {
			dataMap[r.ScoreRange] = make(map[string]*RegimeStats)
		}
		s := &RegimeStats{TotalTrades: r.TotalTrades, WinTrades: r.WinTrades, TotalPnL: r.TotalPnL}
		if s.TotalTrades > 0 {
			s.WinRate = float64(s.WinTrades) / float64(s.TotalTrades)
			s.AvgPnL = s.TotalPnL / float64(s.TotalTrades)
		}
		dataMap[r.ScoreRange][r.Regime] = s
	}

	// 按顺序返回: 80-100, 60-80, 40-60, 0-40
	scoreRangeOrder := []string{"80-100", "60-80", "40-60", "0-40"}
	allRegimes := []string{"顺势", "逆势", "震荡"}

	result := make([]ScoreGradeRegimeResult, 0, len(scoreRangeOrder))
	for _, scoreRange := range scoreRangeOrder {
		regimesMap := make(map[string]RegimeStats)
		if regimes, ok := dataMap[scoreRange]; ok {
			for _, regime := range allRegimes {
				if stats, ok := regimes[regime]; ok {
					regimesMap[regime] = *stats
				}
			}
		}
		result = append(result, ScoreGradeRegimeResult{
			ScoreRange: scoreRange,
			Regimes:    regimesMap,
		})
	}
	return result, nil
}
