package handler

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/internal/repository"
	"github.com/smallfire/starfire/internal/service/market"
	"go.uber.org/zap"
)

// MarketHandler 市场API处理器
type MarketHandler struct {
	marketRepo   repository.MarketRepo
	symbolRepo   repository.SymbolRepo
	klineRepo    repository.KlineRepo
	trendRepo    repository.TrendRepo
	limitStatRepo repository.LimitStatRepo
	factory      *market.Factory
	logger       *zap.Logger
}

// NewMarketHandler 创建市场API处理器
func NewMarketHandler(marketRepo repository.MarketRepo, symbolRepo repository.SymbolRepo, klineRepo repository.KlineRepo, trendRepo repository.TrendRepo, limitStatRepo repository.LimitStatRepo, factory *market.Factory, logger *zap.Logger) *MarketHandler {
	return &MarketHandler{
		marketRepo:   marketRepo,
		symbolRepo:   symbolRepo,
		klineRepo:    klineRepo,
		trendRepo:    trendRepo,
		limitStatRepo: limitStatRepo,
		factory:      factory,
		logger:       logger,
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
	Trend4h     string   `json:"trend_4h"`
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

// AStockIndex A股大盘指数
type AStockIndex struct {
	Code  string  `json:"code"`  // 上证: sh000001, 深证: sz399001, 创业板: sz399006
	Name  string  `json:"name"` // 指数名称
	Price float64 `json:"price"`
	Change float64 `json:"change"`   // 涨跌幅 (%)
	ChangeAmt float64 `json:"change_amt"` // 涨跌额
}

// SectorInfo 板块信息
type SectorInfo struct {
	Code     string  `json:"code"`
	Name     string  `json:"name"`
	Change   float64 `json:"change"`   // 涨跌幅 (%)
	LeadStock string `json:"lead_stock"` // 领涨股
	LeadChange float64 `json:"lead_change"` // 领涨股涨跌幅
}

// AStockOverview A股行情概览
type AStockOverview struct {
	Indices     []AStockIndex  `json:"indices"`     // 大盘指数
	UpSectors   []SectorInfo   `json:"up_sectors"`  // 涨幅板块
	DownSectors []SectorInfo   `json:"down_sectors"`// 跌幅板块
	UpLimitCount int           `json:"up_limit_count"` // 涨停数
	DownLimitCount int         `json:"down_limit_count"` // 跌停数
}

// GetAStockOverview 获取A股行情概览
func (h *MarketHandler) GetAStockOverview(c *gin.Context) {
	overview := AStockOverview{}

	// 从 factory 获取 A 股抓取器及其备选列表
	if h.factory != nil {
		fetchers := h.factory.GetFetchersWithFallback("a_stock")

		// 获取大盘指数（尝试每个抓取器直到成功）
		for _, fetcher := range fetchers {
			if indices, err := fetcher.FetchAStockIndices(); err == nil && len(indices) > 0 {
				for _, idx := range indices {
					overview.Indices = append(overview.Indices, AStockIndex{
						Code:      idx.Code,
						Name:      idx.Name,
						Price:     idx.Price,
						Change:    idx.Change,
						ChangeAmt: idx.ChangeAmt,
					})
				}
				break // 成功获取后停止尝试其他抓取器
			}
		}

		// 获取涨幅板块（按涨跌幅降序取前5）
		for _, fetcher := range fetchers {
			if sectors, err := fetcher.FetchSectorList("f3", false, 5); err == nil && len(sectors) > 0 {
				for _, s := range sectors {
					overview.UpSectors = append(overview.UpSectors, SectorInfo{
						Code:   s.Code,
						Name:   s.Name,
						Change: s.Change,
					})
				}
				break
			}
		}

		// 获取跌幅板块（按涨跌幅升序取前5）
		for _, fetcher := range fetchers {
			if sectors, err := fetcher.FetchSectorList("f3", true, 5); err == nil && len(sectors) > 0 {
				for _, s := range sectors {
					overview.DownSectors = append(overview.DownSectors, SectorInfo{
						Code:   s.Code,
						Name:   s.Name,
						Change: s.Change,
					})
				}
				break
			}
		}

		// 获取涨跌停统计（尝试每个抓取器直到成功）
		for _, fetcher := range fetchers {
			limitCount, err := fetcher.FetchLimitCount()
			if err == nil && (limitCount.UpCount > 0 || limitCount.DownCount > 0) {
				overview.UpLimitCount = limitCount.UpCount
				overview.DownLimitCount = limitCount.DownCount

				// 异步保存涨跌停数据到数据库
				if h.limitStatRepo != nil {
					go func(upCount, downCount int) {
						stat := &models.AStockLimitStat{
							TradeDate:      time.Now().UTC().Truncate(24 * time.Hour),
							UpLimitCount:   upCount,
							DownLimitCount: downCount,
						}
						if err := h.limitStatRepo.Upsert(stat); err != nil {
							h.logger.Warn("保存涨跌停统计失败", zap.Error(err))
						}
					}(limitCount.UpCount, limitCount.DownCount)
				}

				break
			}
		}
	}

	HandleSuccess(c, overview)
}

// GetIndexKlines 获取指数K线数据（用于成交量图表）
// indexCode: "sh000001" (上证), "sz399001" (深证), "sz399006" (创业板)
// period: "daily", "weekly", "monthly"
// limit: 返回数量，默认30
func (h *MarketHandler) GetIndexKlines(c *gin.Context) {
	indexCode := c.DefaultQuery("index_code", "sh000001")
	period := c.DefaultQuery("period", "daily")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "30"))
	if limit <= 0 || limit > 100 {
		limit = 30
	}

	type IndexKline struct {
		Date   string  `json:"date"`
		Open   float64 `json:"open"`
		Close  float64 `json:"close"`
		High   float64 `json:"high"`
		Low    float64 `json:"low"`
		Volume float64 `json:"volume"`
	}

	// 优先从本地数据库读取
	if h.symbolRepo != nil {
		symbol, err := h.symbolRepo.FindByCode("a_stock", indexCode)
		if err == nil && symbol != nil {
			// period 映射：前端传 daily/weekly/monthly -> 数据库存 1d
			dbPeriod := "1d"
			switch period {
			case "weekly":
				dbPeriod = "1w"
			case "monthly":
				dbPeriod = "1mo"
			}

			klines, err := h.klineRepo.GetLatestN(symbol.ID, dbPeriod, limit)
			if err == nil && len(klines) > 0 {
				cst := time.FixedZone("CST", 8*3600)
				var result []IndexKline
				for _, k := range klines {
					result = append(result, IndexKline{
						Date:   k.OpenTime.In(cst).Format("2006-01-02"),
						Open:   k.OpenPrice,
						Close:  k.ClosePrice,
						High:   k.HighPrice,
						Low:    k.LowPrice,
						Volume: k.Volume,
					})
				}
				HandleSuccess(c, gin.H{
					"index_code": indexCode,
					"period":     period,
					"klines":     result,
				})
				return
			}
		}
	}

	// 数据库无数据时，回退到外部 API
	if h.factory != nil {
		fetchers := h.factory.GetFetchersWithFallback("a_stock")

		for _, fetcher := range fetchers {
			klines, err := fetcher.FetchIndexKlines(indexCode, period, limit)
			if err == nil && len(klines) > 0 {
				var result []IndexKline
				for _, k := range klines {
					localTime := k.OpenTime.In(time.FixedZone("CST", 8*3600))
					result = append(result, IndexKline{
						Date:   localTime.Format("2006-01-02"),
						Open:   k.Open,
						Close:  k.Close,
						High:   k.High,
						Low:    k.Low,
						Volume: k.Volume,
					})
				}

				HandleSuccess(c, gin.H{
					"index_code": indexCode,
					"period":     period,
					"klines":     result,
				})
				return
			}
		}
	}

	HandleError(c, http.StatusInternalServerError, fmt.Errorf("无法获取指数K线数据"))
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
		Trend4h:    sym.GetTrend4h(),
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


// LimitStatItem 涨跌停统计数据项
type LimitStatItem struct {
	Date          string `json:"date"`
	UpLimitCount   int    `json:"up_limit_count"`
	DownLimitCount int    `json:"down_limit_count"`
}

// GetLimitStats 获取A股涨跌停历史统计
func (h *MarketHandler) GetLimitStats(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "10"))
	if days <= 0 || days > 30 {
		days = 10
	}

	if h.limitStatRepo == nil {
		HandleError(c, http.StatusInternalServerError, fmt.Errorf("涨跌停统计服务未启用"))
		return
	}

	stats, err := h.limitStatRepo.GetRecent(days)
	if err != nil {
		h.logger.Error("获取涨跌停统计失败", zap.Error(err))
		HandleError(c, http.StatusInternalServerError, err)
		return
	}

	// 转换为前端格式
	cst := time.FixedZone("CST", 8*3600)
	items := make([]LimitStatItem, 0, len(stats))
	for _, s := range stats {
		items = append(items, LimitStatItem{
			Date:          s.TradeDate.In(cst).Format("2006-01-02"),
			UpLimitCount:  s.UpLimitCount,
			DownLimitCount: s.DownLimitCount,
		})
	}

	HandleSuccess(c, gin.H{
		"days":  days,
		"count": len(items),
		"items": items,
	})
}
