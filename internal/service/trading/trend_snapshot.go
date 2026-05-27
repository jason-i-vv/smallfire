package trading

import (
	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/internal/repository"
	trendcalc "github.com/smallfire/starfire/internal/service/trend"
)

func populateTrendSnapshot(track *models.TradeTrack, klineRepo repository.KlineRepo) {
	if track == nil || klineRepo == nil {
		return
	}
	track.Trend4h = calculateTrendSnapshot(klineRepo, track.SymbolID, "4h")
	track.Trend1h = calculateTrendSnapshot(klineRepo, track.SymbolID, "1h")
	track.Trend15m = calculateTrendSnapshot(klineRepo, track.SymbolID, "15m")
}

func calculateTrendSnapshot(klineRepo repository.KlineRepo, symbolID int, period string) string {
	klines, err := klineRepo.GetLatestN(symbolID, period, 60)
	if err != nil || len(klines) < 30 {
		return models.TrendTypeSideways
	}
	trendType, _ := trendcalc.CalculateFromKlines(klines)
	return trendType
}
