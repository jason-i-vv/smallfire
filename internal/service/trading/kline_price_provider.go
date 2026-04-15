package trading

import (
	"github.com/smallfire/starfire/internal/repository"
)

// KlinePriceProvider 基于 K 线数据的价格提供者
// 从数据库最新 K 线获取收盘价，用于 PositionMonitor 止损止盈检查
type KlinePriceProvider struct {
	klineRepo repository.KlineRepo
}

// NewKlinePriceProvider 创建基于 K 线的价格提供者
func NewKlinePriceProvider(klineRepo repository.KlineRepo) *KlinePriceProvider {
	return &KlinePriceProvider{klineRepo: klineRepo}
}

// GetCurrentPrice 获取标的最新收盘价
// 按优先级尝试常用周期：1h → 15m → 1d
func (p *KlinePriceProvider) GetCurrentPrice(symbolID int) (float64, error) {
	periods := []string{"1h", "15m", "1d"}
	for _, period := range periods {
		klines, err := p.klineRepo.GetLatestN(symbolID, period, 1)
		if err != nil {
			continue
		}
		if len(klines) > 0 && klines[0].ClosePrice > 0 {
			return klines[0].ClosePrice, nil
		}
	}
	return 0, nil
}
