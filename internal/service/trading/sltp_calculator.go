package trading

import (
	"time"

	"github.com/smallfire/starfire/internal/config"
	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/internal/repository"
	"github.com/smallfire/starfire/internal/service/strategy"
	"github.com/smallfire/starfire/internal/service/strategy/helpers"
	"go.uber.org/zap"
)

// CalcSLTP 计算止盈止损价格
// 优先级：1. opportunity 建议值 → 2. ATR 动态计算 → 3. 固定百分比兜底
// 优化：震荡市放宽止损、收窄止盈；量比异常收紧止损
func CalcSLTP(entryPrice float64, opp *models.TradingOpportunity, cfg *config.TradingConfig, klineRepo repository.KlineRepo, logger *zap.Logger) (float64, float64) {
	sl, tp := 0.0, 0.0

	// 根据市场状态动态调整止盈止损比例
	slPct, tpPct := GetRegimeSLTP(opp, cfg)

	// 1. 尝试使用 opportunity 建议值
	if opp.SuggestedStopLoss != nil && *opp.SuggestedStopLoss > 0 {
		minDist := entryPrice * 0.01
		if opp.Direction == models.DirectionLong && *opp.SuggestedStopLoss < entryPrice-minDist {
			sl = *opp.SuggestedStopLoss
		} else if opp.Direction == models.DirectionShort && *opp.SuggestedStopLoss > entryPrice+minDist {
			sl = *opp.SuggestedStopLoss
		}
	}
	if opp.SuggestedTakeProfit != nil && *opp.SuggestedTakeProfit > 0 {
		minDist := entryPrice * 0.01
		if opp.Direction == models.DirectionLong && *opp.SuggestedTakeProfit > entryPrice+minDist {
			tp = *opp.SuggestedTakeProfit
		} else if opp.Direction == models.DirectionShort && *opp.SuggestedTakeProfit < entryPrice-minDist {
			tp = *opp.SuggestedTakeProfit
		}
	}

	// 2. 如果建议值不完整，用 ATR 动态计算
	if sl == 0 || tp == 0 {
		atrSL, atrTP := CalcATRSLTP(entryPrice, opp, cfg, klineRepo, logger)
		if sl == 0 && atrSL > 0 {
			sl = atrSL
		}
		if tp == 0 && atrTP > 0 {
			tp = atrTP
		}
	}

	// 3. 兜底：动态百分比（基于市场状态）
	if sl == 0 {
		if opp.Direction == models.DirectionLong {
			sl = entryPrice * (1 - slPct)
		} else {
			sl = entryPrice * (1 + slPct)
		}
	}
	if tp == 0 {
		if opp.Direction == models.DirectionLong {
			tp = entryPrice * (1 + tpPct)
		} else {
			tp = entryPrice * (1 - tpPct)
		}
	}

	return sl, tp
}

// GetRegimeSLTP 根据市场状态返回止盈止损比例
// 震荡市：SL=3%, TP=3%, RR=1:1（价格来回波动，止损太紧容易被扫，止盈太远到不了）
// 趋势市：SL=2%, TP=5%, RR=2.5:1（趋势明确，可以给更大止盈空间）
// 量比异常(>10x)：SL=1.5%, TP=3%（高波动期收紧止损）
func GetRegimeSLTP(opp *models.TradingOpportunity, cfg *config.TradingConfig) (float64, float64) {
	slPct := cfg.StopLossPercent   // 默认 0.02
	tpPct := cfg.TakeProfitPercent // 默认 0.05

	// 检查量比是否异常
	volumeRatio := 0.0
	if opp.ScoreDetails != nil {
		if vr, ok := (*opp.ScoreDetails)["volume_ratio"]; ok {
			if f, ok := vr.(float64); ok {
				volumeRatio = f
			}
		}
	}

	if volumeRatio > 10.0 {
		// 异常放量：收紧止损和止盈
		slPct = 0.015
		tpPct = 0.03
	} else if opp.Regime == "震荡" {
		// 震荡市：放宽止损到3%，收窄止盈到3%
		slPct = 0.03
		tpPct = 0.03
	}
	// 趋势市（顺势/逆势）使用默认值

	return slPct, tpPct
}

// CalcATRSLTP 基于近期K线的ATR计算止盈止损
func CalcATRSLTP(entryPrice float64, opp *models.TradingOpportunity, cfg *config.TradingConfig, klineRepo repository.KlineRepo, logger *zap.Logger) (float64, float64) {
	atrPeriod := cfg.ATRPeriod
	if atrPeriod <= 0 {
		atrPeriod = 14
	}
	atrMultiplier := cfg.ATRMultiplier
	if atrMultiplier <= 0 {
		atrMultiplier = 2.0
	}
	rrRatio := cfg.MinRiskRewardRatio
	if rrRatio <= 0 {
		rrRatio = 1.5
	}

	// 拉取K线数据（需要 atrPeriod+1 根）
	periods := []string{opp.Period, "15m", "1h"}
	var klines []models.Kline
	for _, p := range periods {
		if p == "" {
			continue
		}
		ks, err := klineRepo.GetLatestN(opp.SymbolID, p, atrPeriod+1)
		if err != nil || len(ks) < 2 {
			continue
		}
		klines = ks
		break
	}

	if len(klines) < 2 {
		logger.Debug("ATR K线数据不足，回退固定百分比",
			zap.Int("symbol_id", opp.SymbolID))
		return 0, 0
	}

	atr := helpers.CalculateATR(klines, atrPeriod)
	if atr <= 0 {
		return 0, 0
	}

	sl, tp := strategy.CalculateSLTP(entryPrice, opp.Direction, atr, atrMultiplier, rrRatio)

	logger.Info("ATR 动态止盈止损",
		zap.String("symbol_code", opp.SymbolCode),
		zap.Float64("entry_price", entryPrice),
		zap.Float64("atr", atr),
		zap.Float64("atr_pct", atr/entryPrice*100),
		zap.Float64("stop_loss", sl),
		zap.Float64("take_profit", tp),
		zap.Float64("sl_pct", func() float64 {
			if opp.Direction == models.DirectionLong {
				return (entryPrice - sl) / entryPrice * 100
			}
			return (sl - entryPrice) / entryPrice * 100
		}()),
		zap.Float64("tp_pct", func() float64 {
			if opp.Direction == models.DirectionLong {
				return (tp - entryPrice) / entryPrice * 100
			}
			return (entryPrice - tp) / entryPrice * 100
		}()))

	return sl, tp
}

// ValidateRiskReward 校验并调整止盈止损的盈亏比
func ValidateRiskReward(entryPrice float64, direction string, stopLoss, takeProfit float64, minRR float64) (float64, float64) {
	if minRR <= 0 {
		minRR = 1.5
	}

	var slDist, tpDist float64
	if direction == models.DirectionLong {
		slDist = entryPrice - stopLoss
		tpDist = takeProfit - entryPrice
	} else {
		slDist = stopLoss - entryPrice
		tpDist = entryPrice - takeProfit
	}

	if slDist <= 0 || tpDist <= 0 {
		return stopLoss, takeProfit
	}

	actualRR := tpDist / slDist
	if actualRR < minRR {
		newTPDist := slDist * minRR
		if direction == models.DirectionLong {
			takeProfit = entryPrice + newTPDist
		} else {
			takeProfit = entryPrice - newTPDist
		}
	}

	return stopLoss, takeProfit
}

// MaxStopLossDistance 返回最大允许的止损距离（相对于入场价的比例）
func MaxStopLossDistance(cfg *config.TradingConfig) float64 {
	if cfg.MaxStopLossPercent > 0 {
		return cfg.MaxStopLossPercent
	}
	return 0.05 // 默认 5%
}

// GetEntryPriceAndTime 获取入场价格和时间
// 信号在 K 线收盘后产生，实际只能在下一根 K 线的开盘价入场
func GetEntryPriceAndTime(opp *models.TradingOpportunity, klineRepo repository.KlineRepo) (float64, time.Time) {
	periods := []string{opp.Period, "1h", "15m", "1d"}
	for _, period := range periods {
		if period == "" {
			continue
		}
		klines, err := klineRepo.GetLatestN(opp.SymbolID, period, 2)
		if err != nil || len(klines) == 0 {
			continue
		}

		latest := klines[0] // GetLatestN 按 open_time DESC 返回，[0] 是最新的

		// 优先使用未收盘 K 线的开盘价（即信号产生后的下一根 K 线）
		if !latest.IsClosed && latest.OpenPrice > 0 {
			return latest.OpenPrice, latest.OpenTime
		}

		// 兜底：如果最新 K 线已收盘，说明下一根还未入库，用收盘价
		if latest.ClosePrice > 0 {
			return latest.ClosePrice, latest.CloseTime
		}
	}
	return 0, time.Time{}
}
