package trading

import (
	"math"
	"time"

	"github.com/smallfire/starfire/internal/config"
	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/internal/repository"
)

// StatisticsService 交易统计分析服务
type StatisticsService struct {
	trackRepo repository.TradeTrackRepo
	config    *config.TradingConfig
}

// NewStatisticsService 创建统计分析服务实例
func NewStatisticsService(trackRepo repository.TradeTrackRepo, cfg *config.TradingConfig) *StatisticsService {
	return &StatisticsService{
		trackRepo: trackRepo,
		config:    cfg,
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
	tracks, _ := s.trackRepo.GetClosedTracks(startDate, endDate)
	return s.calculateStatistics(tracks)
}

func (s *StatisticsService) calculateStatistics(tracks []*models.TradeTrack) (*TradeStatistics, error) {
	stats := &TradeStatistics{
		InitialCapital: s.config.InitialCapital,
	}

	if len(tracks) == 0 {
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

	// 暂时不计算 SharpeRatio 和 CalmarRatio（需要更多数据）
	stats.SharpeRatio = 0
	stats.CalmarRatio = 0

	return stats, nil
}

// SignalAnalysis 信号分析统计
type SignalAnalysis struct {
	SignalType  string  `json:"signal_type"`
	TotalTrades int     `json:"total_trades"`
	WinTrades   int     `json:"win_trades"`
	WinRate     float64 `json:"win_rate"`
	TotalPnL    float64 `json:"total_pnl"`
}

// GetSignalAnalysis 按信号类型分析
func (s *StatisticsService) GetSignalAnalysis() (map[string]*SignalAnalysis, error) {
	tracks, _ := s.trackRepo.GetClosedTracks(nil, nil)

	analysis := make(map[string]*SignalAnalysis)

	for _, track := range tracks {
		signalType := s.getSignalType(track)
		if _, ok := analysis[signalType]; !ok {
			analysis[signalType] = &SignalAnalysis{
				SignalType: signalType,
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
	// 暂时返回默认类型，实际需要根据 SignalID 关联查询
	return "unknown"
}
