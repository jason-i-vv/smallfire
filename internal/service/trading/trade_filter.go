package trading

import (
	"github.com/smallfire/starfire/internal/models"
)

// ShouldTrade 交易质量过滤
// 基于80分以上交易的历史数据分析，过滤掉低胜率的交易机会
func ShouldTrade(opp *models.TradingOpportunity) bool {
	// 1. 策略历史胜率 < 50% 的信号不自动交易
	// 数据显示：42.5%和46.9%胜率的信号拿到了80+分却亏损
	winRate := getWinRateFromScoreDetails(opp.ScoreDetails)
	if winRate > 0 && winRate < 0.5 {
		return false
	}

	// 2. 单信号(consensus=1)不自动交易
	// 数据显示：confluence_count=1 的80+分交易胜率仅35%
	if opp.SignalCount <= 1 {
		return false
	}

	// 3. 震荡市做空需要更高阈值
	// 数据显示：做空整体胜率仅23%，震荡市做空更差
	if opp.Regime == "震荡" && opp.Direction == models.DirectionShort && opp.Score < 90 {
		return false
	}

	return true
}

// getWinRateFromScoreDetails 从评分明细中提取策略历史胜率
func getWinRateFromScoreDetails(details *models.JSONB) float64 {
	if details == nil {
		return 0
	}
	if vr, ok := (*details)["win_rate_input"]; ok {
		if f, ok := vr.(float64); ok {
			return f
		}
	}
	return 0
}
