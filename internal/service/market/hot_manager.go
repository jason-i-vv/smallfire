package market

import (
	"sort"
	"time"

	"github.com/smallfire/starfire/internal/config"
	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/internal/repository"
	"go.uber.org/zap"
)

// HotManager 热度管理器
type HotManager struct {
	symbolRepo repository.SymbolRepo
	marketRepo repository.MarketRepo
	factory    *Factory
	config     *config.MarketsConfig
	logger     *zap.Logger
}

// NewHotManager 创建热度管理器
func NewHotManager(symbolRepo repository.SymbolRepo,
	marketRepo repository.MarketRepo, factory *Factory,
	cfg *config.MarketsConfig, logger *zap.Logger) *HotManager {
	return &HotManager{
		symbolRepo: symbolRepo,
		marketRepo: marketRepo,
		factory:    factory,
		config:     cfg,
		logger:     logger,
	}
}

// UpdateHotSymbols 更新热度标的
func (m *HotManager) UpdateHotSymbols() error {
	enabledFetchers := m.factory.ListEnabledFetchers()

	for _, fetcher := range enabledFetchers {
		if err := m.updateMarketHot(fetcher); err != nil {
			m.logger.Error("更新热度标的失败",
				zap.String("market", fetcher.MarketCode()),
				zap.Error(err))
		}
	}

	return nil
}

func (m *HotManager) updateMarketHot(fetcher Fetcher) error {
	marketCode := fetcher.MarketCode()
	limit := m.getLimit(marketCode)
	hotDays := m.getHotDays(marketCode)

	// 获取所有交易对
	symbols, err := fetcher.FetchSymbols()
	if err != nil {
		m.logger.Error("获取交易对列表失败",
			zap.String("market", marketCode),
			zap.Error(err))
		return err
	}

	// 模拟按热度排序（这里简化处理，后续可根据成交量、成交额等计算热度）
	// 暂时随机设置热度分数
	for i := range symbols {
		symbols[i].HotScore = float64(len(symbols) - i)
	}

	// 按热度排序
	sort.Slice(symbols, func(i, j int) bool {
		return symbols[i].HotScore > symbols[j].HotScore
	})

	// 取前N名
	if len(symbols) > limit {
		symbols = symbols[:limit]
	}

	// 获取市场ID
	market, err := m.marketRepo.FindByCode(marketCode)
	if err != nil {
		m.logger.Error("获取市场信息失败",
			zap.String("market", marketCode),
			zap.Error(err))
		return err
	}

	// 更新数据库
	now := time.Now()
	createdCount := 0
	updatedCount := 0
	for _, sym := range symbols {
		// 查找或创建标的
		symbol, err := m.symbolRepo.FindByCode(marketCode, sym.Code)
		if err != nil {
			// 创建新标的
			symbol = &models.Symbol{
				MarketID:       market.ID,
				MarketCode:     marketCode,
				SymbolCode:     sym.Code,
				SymbolName:     sym.Name,
				SymbolType:     sym.Type,
				IsTracking:     true,
				MaxKlinesCount: 1000,
				HotScore:       sym.HotScore,
				LastHotAt:      &now,
			}
			if err := m.symbolRepo.Create(symbol); err != nil {
				m.logger.Error("创建标的失败",
					zap.String("code", sym.Code),
					zap.Error(err))
				continue
			}
			createdCount++
		} else {
			// 更新热度
			symbol.HotScore = sym.HotScore
			symbol.LastHotAt = &now
			symbol.IsTracking = true
			symbol.SymbolName = sym.Name
			symbol.SymbolType = sym.Type
			if err := m.symbolRepo.Update(symbol); err != nil {
				m.logger.Error("更新标的失败",
					zap.String("code", sym.Code),
					zap.Error(err))
				continue
			}
			updatedCount++
		}
	}

	// 清理过期标的（超过N天无热度更新）
	cutoff := now.AddDate(0, 0, -hotDays)
	if err := m.symbolRepo.DisableExpiredHot(cutoff); err != nil {
		m.logger.Error("清理过期热度标的失败", zap.Error(err))
	}

	m.logger.Info("热度标的更新完成",
		zap.String("market", marketCode),
		zap.Int("total", createdCount+updatedCount))

	return nil
}

func (m *HotManager) getLimit(marketCode string) int {
	switch marketCode {
	case "bybit":
		return m.config.Bybit.SymbolsLimit
	case "a_stock":
		return m.config.AStock.SymbolsLimit
	case "us_stock":
		return m.config.USStock.SymbolsLimit
	default:
		return 200
	}
}

func (m *HotManager) getHotDays(marketCode string) int {
	return 30 // 默认30天
}
