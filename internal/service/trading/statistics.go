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
func (s *StatisticsService) GetStatistics(startDate, endDate *time.Time) (*TradeStatistics, error) {
	tracks, err := s.trackRepo.GetClosedTracks(startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("获取已平仓记录失败: %w", err)
	}
	return s.calculateStatistics(tracks)
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
func (s *StatisticsService) GetSignalAnalysis() (map[string]*SignalAnalysis, error) {
	tracks, err := s.trackRepo.GetClosedTracks(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("获取已平仓记录失败: %w", err)
	}

	analysis := make(map[string]*SignalAnalysis)

	for _, track := range tracks {
		signalType, sourceType := s.getFullSignalInfo(track)
		if _, ok := analysis[signalType]; !ok {
			analysis[signalType] = &SignalAnalysis{
				SignalType: signalType,
				SourceType: sourceType,
			}
		}

		a := analysis[signalType]
		a.TotalTrades++
		if track.PnL != nil && *track.PnL > 0 {
			a.WinTrades++
			a.TotalPnL += *track.PnL
		} else if track.PnL != nil {
			a.TotalPnL += *track.PnL
		}
	}

	// 计算胜率
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
func (s *StatisticsService) GetEquityCurve(startDate, endDate *time.Time) ([]EquityCurvePoint, error) {
	tracks, err := s.trackRepo.GetClosedTracks(startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("获取已平仓记录失败: %w", err)
	}

	// 按 ExitTime 排序
	sort.Slice(tracks, func(i, j int) bool {
		if tracks[i].ExitTime == nil || tracks[j].ExitTime == nil {
			return false
		}
		return tracks[i].ExitTime.Before(*tracks[j].ExitTime)
	})

	// 使用 map 去重，同一时间戳只保留最后一个权益值
	equityByTime := make(map[int64]float64)
	equity := s.config.InitialCapital

	// 起始点
	if len(tracks) > 0 && tracks[0].ExitTime != nil {
		equityByTime[tracks[0].ExitTime.Add(-time.Minute).Unix()] = equity
	}

	for _, track := range tracks {
		if track.PnL != nil {
			equity += *track.PnL
		}
		if track.ExitTime != nil {
			equityByTime[track.ExitTime.Unix()] = equity
		}
	}

	// 转换为有序切片
	points := make([]EquityCurvePoint, 0, len(equityByTime))
	for time, equity := range equityByTime {
		points = append(points, EquityCurvePoint{Time: time, Equity: equity})
	}
	sort.Slice(points, func(i, j int) bool {
		return points[i].Time < points[j].Time
	})

	return points, nil
}

// GetSymbolAnalysis 按标的统计
func (s *StatisticsService) GetSymbolAnalysis(startDate, endDate *time.Time) ([]SymbolAnalysis, error) {
	tracks, err := s.trackRepo.GetClosedTracks(startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("获取已平仓记录失败: %w", err)
	}

	groups := make(map[int]*SymbolAnalysis)

	for _, track := range tracks {
		if track.PnL == nil {
			continue
		}
		sid := track.SymbolID
		if _, ok := groups[sid]; !ok {
			groups[sid] = &SymbolAnalysis{SymbolID: sid}
		}
		g := groups[sid]
		g.TotalTrades++
		pnl := *track.PnL
		g.TotalPnL += pnl
		if pnl > 0 {
			g.WinTrades++
		}
	}

	// 查找 SymbolCode
	result := make([]SymbolAnalysis, 0, len(groups))
	for sid, g := range groups {
		if s.symbolRepo != nil {
			sym, err := s.symbolRepo.GetByID(sid)
			if err == nil && sym != nil {
				g.SymbolCode = sym.SymbolCode
			}
		}
		if g.TotalTrades > 0 {
			g.WinRate = float64(g.WinTrades) / float64(g.TotalTrades)
			g.AvgPnL = g.TotalPnL / float64(g.TotalTrades)
		}
		result = append(result, *g)
	}

	// 按总盈亏降序
	sort.Slice(result, func(i, j int) bool {
		return result[i].TotalPnL > result[j].TotalPnL
	})

	return result, nil
}

// GetDirectionAnalysis 按方向统计
func (s *StatisticsService) GetDirectionAnalysis(startDate, endDate *time.Time) (map[string]*DirectionAnalysis, error) {
	tracks, err := s.trackRepo.GetClosedTracks(startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("获取已平仓记录失败: %w", err)
	}

	analysis := map[string]*DirectionAnalysis{
		"long":  {Direction: "long"},
		"short": {Direction: "short"},
	}

	for _, track := range tracks {
		if track.PnL == nil {
			continue
		}
		dir := track.Direction
		if dir != "long" && dir != "short" {
			continue
		}
		a := analysis[dir]
		a.TotalTrades++
		pnl := *track.PnL
		a.TotalPnL += pnl
		if pnl > 0 {
			a.WinTrades++
		}
		if track.EntryTime != nil && track.ExitTime != nil {
			a.AvgHoldingHours += track.ExitTime.Sub(*track.EntryTime).Hours()
		}
	}

	for _, a := range analysis {
		if a.TotalTrades > 0 {
			a.WinRate = float64(a.WinTrades) / float64(a.TotalTrades)
			a.AvgPnL = a.TotalPnL / float64(a.TotalTrades)
			a.AvgHoldingHours /= float64(a.TotalTrades)
		}
	}

	return analysis, nil
}

// GetExitReasonAnalysis 按出场原因统计
func (s *StatisticsService) GetExitReasonAnalysis(startDate, endDate *time.Time) ([]ExitReasonAnalysis, error) {
	tracks, err := s.trackRepo.GetClosedTracks(startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("获取已平仓记录失败: %w", err)
	}

	groups := make(map[string]*ExitReasonAnalysis)

	for _, track := range tracks {
		if track.PnL == nil {
			continue
		}
		reason := "unknown"
		if track.ExitReason != nil {
			reason = *track.ExitReason
		}
		if _, ok := groups[reason]; !ok {
			groups[reason] = &ExitReasonAnalysis{ExitReason: reason}
		}
		g := groups[reason]
		g.TotalTrades++
		pnl := *track.PnL
		g.TotalPnL += pnl
		if pnl > 0 {
			g.WinTrades++
		}
	}

	result := make([]ExitReasonAnalysis, 0, len(groups))
	for _, g := range groups {
		if g.TotalTrades > 0 {
			g.WinRate = float64(g.WinTrades) / float64(g.TotalTrades)
		}
		result = append(result, *g)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].TotalTrades > result[j].TotalTrades
	})

	return result, nil
}

// GetPeriodPnL 按时间周期统计盈亏
func (s *StatisticsService) GetPeriodPnL(startDate, endDate *time.Time, period string) ([]PeriodPnL, error) {
	tracks, err := s.trackRepo.GetClosedTracks(startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("获取已平仓记录失败: %w", err)
	}

	groups := make(map[int64]*PeriodPnL)

	for _, track := range tracks {
		if track.PnL == nil || track.ExitTime == nil {
			continue
		}
		var periodStart time.Time
		exitTime := *track.ExitTime

		switch period {
		case "weekly":
			weekday := int(exitTime.Weekday())
			if weekday == 0 {
				weekday = 7
			}
			periodStart = time.Date(exitTime.Year(), exitTime.Month(), exitTime.Day()-weekday+1, 0, 0, 0, 0, exitTime.Location())
		case "monthly":
			periodStart = time.Date(exitTime.Year(), exitTime.Month(), 1, 0, 0, 0, 0, exitTime.Location())
		default: // daily
			periodStart = time.Date(exitTime.Year(), exitTime.Month(), exitTime.Day(), 0, 0, 0, 0, exitTime.Location())
		}

		key := periodStart.Unix()
		if _, ok := groups[key]; !ok {
			groups[key] = &PeriodPnL{PeriodStart: key}
		}
		g := groups[key]
		g.PnL += *track.PnL
		g.TradeCount++
	}

	result := make([]PeriodPnL, 0, len(groups))
	for _, g := range groups {
		result = append(result, *g)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].PeriodStart < result[j].PeriodStart
	})

	return result, nil
}

// GetPnLDistribution 获取盈亏分布
func (s *StatisticsService) GetPnLDistribution(startDate, endDate *time.Time) (*PnLDistribution, error) {
	tracks, err := s.trackRepo.GetClosedTracks(startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("获取已平仓记录失败: %w", err)
	}

	var pnls []float64
	for _, track := range tracks {
		if track.PnL != nil {
			pnls = append(pnls, *track.PnL)
		}
	}

	if len(pnls) == 0 {
		return &PnLDistribution{Buckets: []PnLBucket{}}, nil
	}

	// 找最大最小值
	minPnL, maxPnL := pnls[0], pnls[0]
	for _, p := range pnls {
		if p < minPnL {
			minPnL = p
		}
		if p > maxPnL {
			maxPnL = p
		}
	}

	// 生成 20 个桶
	bucketCount := 20
	rangeSize := (maxPnL - minPnL) / float64(bucketCount)
	if rangeSize == 0 {
		rangeSize = 1
	}

	buckets := make([]PnLBucket, bucketCount)
	for i := 0; i < bucketCount; i++ {
		buckets[i] = PnLBucket{
			RangeStart: minPnL + float64(i)*rangeSize,
			RangeEnd:   minPnL + float64(i+1)*rangeSize,
			IsWin:      (minPnL + float64(i)*rangeSize) >= 0,
		}
	}

	// 分配 PnL 到桶
	for _, p := range pnls {
		idx := int((p - minPnL) / rangeSize)
		if idx >= bucketCount {
			idx = bucketCount - 1
		}
		if idx < 0 {
			idx = 0
		}
		buckets[idx].Count++
	}

	// 移除空桶（可选，保留以便前端对齐）
	return &PnLDistribution{Buckets: buckets}, nil
}

// GetDetailedSignalAnalysis 按具体信号类型分析
func (s *StatisticsService) GetDetailedSignalAnalysis(startDate, endDate *time.Time) ([]SignalAnalysis, error) {
	tracks, err := s.trackRepo.GetClosedTracks(startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("获取已平仓记录失败: %w", err)
	}

	analysis := make(map[string]*SignalAnalysis)

	for _, track := range tracks {
		signalType, sourceType := s.getFullSignalInfo(track)
		key := signalType
		if _, ok := analysis[key]; !ok {
			analysis[key] = &SignalAnalysis{
				SignalType: signalType,
				SourceType: sourceType,
			}
		}

		a := analysis[key]
		a.TotalTrades++
		if track.PnL != nil {
			if *track.PnL > 0 {
				a.WinTrades++
			}
			a.TotalPnL += *track.PnL
		}
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
	ScoreRange     string  `json:"score_range"`      // 评分区间，如 "80-100"
	TotalTrades    int     `json:"total_trades"`     // 交易次数
	WinTrades      int     `json:"win_trades"`       // 盈利次数
	WinRate        float64 `json:"win_rate"`         // 胜率
	TotalPnL       float64 `json:"total_pnl"`        // 总盈亏
	AvgPnL         float64 `json:"avg_pnl"`          // 平均盈亏
	AvgHoldingHours float64 `json:"avg_holding_hours"` // 平均持仓时长
}

// GetScoreAnalysis 按评分区间统计胜率
func (s *StatisticsService) GetScoreAnalysis(startDate, endDate *time.Time) ([]ScoreAnalysis, error) {
	tracks, err := s.trackRepo.GetClosedTracks(startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("获取已平仓记录失败: %w", err)
	}

	// 定义评分区间
	ranges := []struct {
		name  string
		min   int
		max   int
	}{
		{"80-100", 80, 100},
		{"70-80", 70, 79},
		{"60-70", 60, 69},
		{"50-60", 50, 59},
		{"<50", 0, 49},
	}

	// 初始化每个区间的统计
	stats := make(map[string]*ScoreAnalysis)
	for _, r := range ranges {
		stats[r.name] = &ScoreAnalysis{
			ScoreRange: r.name,
		}
	}

	// 遍历所有交易，按评分分组
	for _, track := range tracks {
		if track.PnL == nil {
			continue
		}

		// 获取评分
		score := 0
		if track.OpportunityID != nil && s.oppRepo != nil {
			opp, err := s.oppRepo.GetByID(*track.OpportunityID)
			if err == nil && opp != nil {
				score = opp.Score
			}
		}

		// 确定区间
		var rangeName string
		for _, r := range ranges {
			if score >= r.min && score <= r.max {
				rangeName = r.name
				break
			}
		}
		if rangeName == "" {
			rangeName = "<50"
		}

		a := stats[rangeName]
		a.TotalTrades++
		pnl := *track.PnL
		a.TotalPnL += pnl
		if pnl > 0 {
			a.WinTrades++
		}
		if track.EntryTime != nil && track.ExitTime != nil {
			a.AvgHoldingHours += track.ExitTime.Sub(*track.EntryTime).Hours()
		}
	}

	// 计算统计指标
	result := make([]ScoreAnalysis, 0, len(ranges))
	for _, r := range ranges {
		a := stats[r.name]
		if a.TotalTrades > 0 {
			a.WinRate = float64(a.WinTrades) / float64(a.TotalTrades)
			a.AvgPnL = a.TotalPnL / float64(a.TotalTrades)
			a.AvgHoldingHours /= float64(a.TotalTrades)
		}
		result = append(result, *a)
	}

	return result, nil
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
