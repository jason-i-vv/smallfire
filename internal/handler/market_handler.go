package handler

import (
	"math"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/smallfire/starfire/internal/repository"
	"go.uber.org/zap"
)

// MarketHandler 市场API处理器
type MarketHandler struct {
	marketRepo repository.MarketRepo
	symbolRepo repository.SymbolRepo
	klineRepo  repository.KlineRepo
	trendRepo  repository.TrendRepo
	logger     *zap.Logger
}

// NewMarketHandler 创建市场API处理器
func NewMarketHandler(marketRepo repository.MarketRepo, symbolRepo repository.SymbolRepo, klineRepo repository.KlineRepo, trendRepo repository.TrendRepo, logger *zap.Logger) *MarketHandler {
	return &MarketHandler{
		marketRepo: marketRepo,
		symbolRepo: symbolRepo,
		klineRepo:  klineRepo,
		trendRepo:  trendRepo,
		logger:     logger,
	}
}

// SymbolOverview 标的总览数据
type SymbolOverview struct {
	SymbolID    int      `json:"symbol_id"`
	SymbolCode  string   `json:"symbol_code"`
	SymbolName  string   `json:"symbol_name"`
	MarketCode  string   `json:"market_code"`
	ClosePrice  *float64 `json:"close_price"`
	OpenPrice   *float64 `json:"open_price"`
	Change      *float64 `json:"change"`
	TrendType   *string  `json:"trend_type"`
	TrendStrength *int   `json:"trend_strength"`
}

// GetMarkets 获取所有市场列表
func (h *MarketHandler) GetMarkets(c *gin.Context) {
	markets, err := h.marketRepo.FindEnabled()
	if err != nil {
		h.logger.Error("获取市场列表失败", zap.Error(err))
		HandleError(c, http.StatusInternalServerError, err)
		return
	}
	HandleSuccess(c, markets)
}

// GetMarket 获取指定市场详情
func (h *MarketHandler) GetMarket(c *gin.Context) {
	marketCode := c.Param("market_code")
	market, err := h.marketRepo.FindByCode(marketCode)
	if err != nil {
		h.logger.Error("获取市场详情失败", zap.String("market_code", marketCode), zap.Error(err))
		HandleError(c, http.StatusInternalServerError, err)
		return
	}
	HandleSuccess(c, market)
}

// GetMarketOverview 获取市场总览（标的列表+价格+趋势）
func (h *MarketHandler) GetMarketOverview(c *gin.Context) {
	marketCode := c.Param("market_code")

	// 分页参数
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	period := c.DefaultQuery("period", "15m")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// 获取该市场的跟踪标的
	symbols, err := h.symbolRepo.GetTrackingByMarket(marketCode)
	if err != nil {
		h.logger.Error("获取市场标的失败", zap.String("market_code", marketCode), zap.Error(err))
		HandleError(c, http.StatusInternalServerError, err)
		return
	}

	total := len(symbols)

	// 分页
	start := (page - 1) * pageSize
	end := start + pageSize
	if start >= total {
		HandleSuccess(c, gin.H{
			"items": []interface{}{},
			"total": total,
			"page":  page,
			"page_size": pageSize,
		})
		return
	}
	if end > total {
		end = total
	}
	pageSymbols := symbols[start:end]

	// 为每个标的获取价格和趋势
	items := make([]SymbolOverview, 0, len(pageSymbols))
	for _, sym := range pageSymbols {
		item := SymbolOverview{
			SymbolID:   sym.ID,
			SymbolCode: sym.SymbolCode,
			SymbolName: sym.SymbolName,
			MarketCode: sym.MarketCode,
		}

		// 获取最新K线
		kline, err := h.klineRepo.GetLatest(int64(sym.ID), period)
		if err == nil && kline != nil {
			item.ClosePrice = &kline.ClosePrice
			item.OpenPrice = &kline.OpenPrice
			if kline.OpenPrice > 0 {
				change := math.Round(((kline.ClosePrice-kline.OpenPrice)/kline.OpenPrice*100)*100) / 100
				item.Change = &change
			}
		}

		// 获取趋势
		trend, err := h.trendRepo.GetActive(sym.ID, period)
		if err == nil && trend != nil {
			item.TrendType = &trend.TrendType
			item.TrendStrength = &trend.Strength
		}

		items = append(items, item)
	}

	HandleSuccess(c, gin.H{
		"items":     items,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}
