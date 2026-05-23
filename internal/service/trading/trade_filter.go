package trading

import (
	"github.com/smallfire/starfire/internal/models"
)

// ShouldTrade 系统模拟交易过滤（基础过滤，用于数据采集）
// 只过滤明显不合理的交易，保留尽可能多的数据用于统计分析
func ShouldTrade(opp *models.TradingOpportunity) bool {
	// 1. 策略历史胜率 < 50% 的信号不自动交易
	winRate := getWinRateFromScoreDetails(opp.ScoreDetails)
	if winRate > 0 && winRate < 0.5 {
		return false
	}

	// 2. 单信号(consensus=1)不自动交易
	if opp.SignalCount <= 1 {
		return false
	}

	return true
}

// ShouldTradeStrict Bybit 模拟仓位严格过滤（追求盈利）
// 在基础过滤之上，额外过滤历史数据证明低胜率的场景
func ShouldTradeStrict(opp *models.TradingOpportunity) bool {
	// 先通过基础过滤
	if !ShouldTrade(opp) {
		return false
	}

	// 3. 震荡市不交易（含做多和做空）
	// 数据显示：震荡市80+分交易0%胜率，10笔全亏，净亏-48.83
	if opp.Regime == "震荡" && opp.Score < 90 {
		return false
	}

	// 4. key_level 策略需要更高评分
	// 数据显示：key_level 占77%交易但胜率仅27.3%，净亏-73.30
	if opp.StrategyType == "key_level" && opp.Score < 85 {
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
