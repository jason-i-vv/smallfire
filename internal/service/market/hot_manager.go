package market

import (
	"fmt"
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
		marketCode := fetcher.MarketCode()

		// A 股市场：先注册大盘指数
		if marketCode == "a_stock" {
			if err := m.ensureIndexSymbols(); err != nil {
				m.logger.Warn("注册大盘指数失败", zap.Error(err))
			}
		}

		if err := m.updateMarketHotWithFallback(marketCode); err != nil {
			m.logger.Warn("市场热度更新失败",
				zap.String("market", marketCode),
				zap.Error(err))
		}
	}

	return nil
}

// ensureIndexSymbols 确保 A 股大盘指数在 symbols 表中
func (m *HotManager) ensureIndexSymbols() error {
	market, err := m.marketRepo.FindByCode("a_stock")
	if err != nil {
		return fmt.Errorf("获取A股市场信息失败: %w", err)
	}

	indices := []struct {
		code string
		name string
	}{
		{"sh000001", "上证指数"},
		{"sz399001", "深证成指"},
		{"sz399006", "创业板指"},
	}

	now := time.Now()
	for _, idx := range indices {
		symbol, err := m.symbolRepo.FindByCode("a_stock", idx.code)
		if err != nil {
			symbol = &models.Symbol{
				MarketID:       market.ID,
				MarketCode:     "a_stock",
				SymbolCode:     idx.code,
				SymbolName:     idx.name,
				SymbolType:     "index",
				IsTracking:     true,
				MaxKlinesCount: 1000,
				HotScore:       0,
				LastHotAt:      &now,
			}
			if err := m.symbolRepo.Create(symbol); err != nil {
				m.logger.Error("创建指数标的失败", zap.String("code", idx.code), zap.Error(err))
				continue
			}
			m.logger.Info("注册指数标的", zap.String("code", idx.code), zap.String("name", idx.name))
		} else if !symbol.IsTracking {
			symbol.IsTracking = true
			symbol.SymbolType = "index"
			_ = m.symbolRepo.Update(symbol)
		}
	}

	return nil
}

// updateMarketHotWithFallback 尝试多数据源降级
func (m *HotManager) updateMarketHotWithFallback(marketCode string) error {
	fetchers := m.factory.GetFetchersWithFallback(marketCode)
	if len(fetchers) == 0 {
		return fmt.Errorf("无可用抓取器: %s", marketCode)
	}

	var lastErr error
	for i, fetcher := range fetchers {
		symbols, err := fetcher.FetchSymbols()
		if err != nil {
			lastErr = err
			m.logger.Warn("数据源不可用，尝试下一个",
				zap.String("market", marketCode),
				zap.Int("source", i+1),
				zap.Error(err))
			continue
		}

		// 成功获取数据，执行更新
		if err := m.doUpdateHot(marketCode, symbols); err != nil {
			lastErr = err
			continue
		}

		m.logger.Info("热度标的更新成功",
			zap.String("market", marketCode),
			zap.Int("source", i+1),
			zap.Int("symbols", len(symbols)))
		return nil
	}

	return lastErr
}

func (m *HotManager) updateMarketHot(fetcher Fetcher) error {
	return m.updateMarketHotWithFallback(fetcher.MarketCode())
}

func (m *HotManager) doUpdateHot(marketCode string, symbols []SymbolInfo) error {
	limit := m.getLimit(marketCode)
	hotDays := m.getHotDays(marketCode)

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
		return fmt.Errorf("获取市场信息失败: %w", err)
	}

	// 更新数据库
	now := time.Now()
	createdCount := 0
	updatedCount := 0
	for _, sym := range symbols {
		symbol, err := m.symbolRepo.FindByCode(marketCode, sym.Code)
		if err != nil {
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

	// 清理过期标的
	cutoff := now.AddDate(0, 0, -hotDays)
	if err := m.symbolRepo.DisableExpiredHot(cutoff); err != nil {
		m.logger.Error("清理过期热度标的失败", zap.Error(err))
	}

	m.logger.Info("热度标的更新完成",
		zap.String("market", marketCode),
		zap.Int("created", createdCount),
		zap.Int("updated", updatedCount))

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
