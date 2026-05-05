package market

import (
	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/internal/repository"
	"go.uber.org/zap"
)

// Trend4hHook 4h趋势同步钩子
// 在4h K线同步完成后，基于价格比较法计算币对趋势并更新到 symbol 表
type Trend4hHook struct {
	klineRepo  repository.KlineRepo
	symbolRepo repository.SymbolRepo
	logger     *zap.Logger
}

// NewTrend4hHook 创建4h趋势同步钩子
func NewTrend4hHook(klineRepo repository.KlineRepo, symbolRepo repository.SymbolRepo, logger *zap.Logger) *Trend4hHook {
	return &Trend4hHook{
		klineRepo:  klineRepo,
		symbolRepo: symbolRepo,
		logger:     logger,
	}
}

// OnKlinesSynced K线同步完成回调
func (h *Trend4hHook) OnKlinesSynced(symbolID int, symbolCode, marketCode, period string) {
	if period != "4h" {
		return
	}

	trend := h.calculateTrend(symbolID)
	if trend == "" {
		return
	}

	if err := h.symbolRepo.UpdateTrend(symbolID, trend); err != nil {
		h.logger.Error("更新币对趋势失败",
			zap.Int("symbol_id", symbolID),
			zap.String("trend", trend),
			zap.Error(err))
		return
	}

	h.logger.Debug("更新币对趋势",
		zap.Int("symbol_id", symbolID),
		zap.String("symbol", symbolCode),
		zap.String("trend_4h", trend))
}

// calculateTrend 基于价格比较法计算趋势
// 多头: 当前价 > 48h前价+1% AND > 16h前价+0.5%
// 空头: 当前价 < 48h前价-1% AND < 16h前价-0.5%
// 否则: 震荡
func (h *Trend4hHook) calculateTrend(symbolID int) string {
	// 获取最近30根4h K线 (30*4h=120h≈5天)
	klines, err := h.klineRepo.GetLatestN(symbolID, "4h", 30)
	if err != nil || len(klines) < 13 {
		return models.TrendTypeSideways
	}

	currentPrice := klines[len(klines)-1].ClosePrice

	// 4h K线: 12根前 = 48h, 4根前 = 16h
	price48h := klines[len(klines)-13].ClosePrice
	price16h := klines[len(klines)-5].ClosePrice

	bullish48h := currentPrice > price48h*1.01
	bullish16h := currentPrice > price16h*1.005
	bearish48h := currentPrice < price48h*0.99
	bearish16h := currentPrice < price16h*0.995

	if bullish48h && bullish16h {
		return models.TrendTypeBullish
	}
	if bearish48h && bearish16h {
		return models.TrendTypeBearish
	}
	return models.TrendTypeSideways
}
