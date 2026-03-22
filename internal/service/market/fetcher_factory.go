package market

import (
	"github.com/smallfire/starfire/internal/config"
	"github.com/smallfire/starfire/internal/repository"
)

// Factory 行情抓取器工厂
type Factory struct {
	fetchers map[string]Fetcher
	config   *config.MarketsConfig
	symbolRepo repository.SymbolRepo
	klineRepo  repository.KlineRepo
}

// NewFactory 创建工厂实例
func NewFactory(cfg *config.MarketsConfig, symbolRepo repository.SymbolRepo, klineRepo repository.KlineRepo) *Factory {
	f := &Factory{
		fetchers: make(map[string]Fetcher),
		config:   cfg,
		symbolRepo: symbolRepo,
		klineRepo: klineRepo,
	}

	// 注册抓取器
	if cfg.Bybit.Enabled {
		f.fetchers["bybit"] = NewBybitFetcher(cfg.Bybit)
	}
	// 这里可以继续添加其他市场的抓取器
	// if cfg.AStock.Enabled {
	//     f.fetchers["a_stock"] = NewAStockFetcher(cfg.AStock)
	// }
	// if cfg.USStock.Enabled {
	//     f.fetchers["us_stock"] = NewUSStockFetcher(cfg.USStock)
	// }

	return f
}

func (f *Factory) GetFetcher(marketCode string) (Fetcher, bool) {
	fetcher, ok := f.fetchers[marketCode]
	return fetcher, ok
}

func (f *Factory) ListEnabledFetchers() []Fetcher {
	var enabled []Fetcher
	for _, fetcher := range f.fetchers {
		enabled = append(enabled, fetcher)
	}
	return enabled
}

func (f *Factory) HasFetcher(marketCode string) bool {
	_, ok := f.fetchers[marketCode]
	return ok
}

func (f *Factory) Count() int {
	return len(f.fetchers)
}
