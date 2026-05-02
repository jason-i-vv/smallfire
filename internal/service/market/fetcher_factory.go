package market

import (
	"github.com/smallfire/starfire/internal/config"
	"github.com/smallfire/starfire/internal/repository"
)

// Factory 行情抓取器工厂
type Factory struct {
	fetchers    map[string]Fetcher
	fallbackers map[string][]Fetcher // 同市场的备选抓取器（按优先级排序）
	config     *config.MarketsConfig
	symbolRepo repository.SymbolRepo
	klineRepo  repository.KlineRepo
}

// NewFactory 创建工厂实例
func NewFactory(cfg *config.MarketsConfig, symbolRepo repository.SymbolRepo, klineRepo repository.KlineRepo) *Factory {
	f := &Factory{
		fetchers:    make(map[string]Fetcher),
		fallbackers: make(map[string][]Fetcher),
		config:     cfg,
		symbolRepo: symbolRepo,
		klineRepo:  klineRepo,
	}

	// 注册主抓取器
	if cfg.Bybit.Enabled {
		f.fetchers["bybit"] = NewBybitFetcher(cfg.Bybit)
	}
	if cfg.AStock.Enabled {
		// A股注册两个数据源，按优先级自动降级
		// 新浪财经优先（数据更稳定），东方财富作为备选
		f.fetchers["a_stock"] = NewSinaFetcher(cfg.AStock)
		f.fallbackers["a_stock"] = []Fetcher{NewEastmoneyFetcher(cfg.AStock)}
	}
	if cfg.USStock.Enabled {
		f.fetchers["us_stock"] = NewYahooFetcher(cfg.USStock)
	}

	return f
}

func (f *Factory) GetFetcher(marketCode string) (Fetcher, bool) {
	fetcher, ok := f.fetchers[marketCode]
	return fetcher, ok
}

// GetFetchersWithFallback 获取主抓取器及其备选列表
func (f *Factory) GetFetchersWithFallback(marketCode string) []Fetcher {
	var result []Fetcher
	if primary, ok := f.fetchers[marketCode]; ok {
		result = append(result, primary)
	}
	if fallbacks, ok := f.fallbackers[marketCode]; ok {
		result = append(result, fallbacks...)
	}
	return result
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
