package market

import (
	"time"

	"github.com/smallfire/starfire/internal/models"
)

// Fetcher 行情抓取器接口
type Fetcher interface {
	// 获取市场代码
	MarketCode() string

	// 获取支持的周期
	SupportedPeriods() []string

	// 获取交易对列表
	FetchSymbols() ([]SymbolInfo, error)

	// 获取K线数据（按数量限制）
	FetchKlines(symbol string, period string, limit int) ([]KlineData, error)

	// 获取指定时间范围的K线数据
	FetchKlinesByTimeRange(symbol string, period string, startTime, endTime time.Time) ([]KlineData, error)

	// 获取实时价格
	FetchTicker(symbol string) (*Ticker, error)

	// 获取A股大盘指数（仅A股有效）
	FetchAStockIndices() ([]AStockMarketIndex, error)

	// 获取板块涨跌榜（仅A股有效）
	// sortField: f3=涨跌幅, f6=成交额; ascending: false=降序
	FetchSectorList(sortField string, ascending bool, limit int) ([]SectorData, error)

	// 获取涨跌停统计（仅A股有效）
	FetchLimitCount() (*LimitCount, error)

	// 获取指数K线数据（仅A股有效，用于成交量图表）
	// indexCode: 上证指数 "sh000001", 深证成指 "sz399001", 创业板 "sz399006"
	// period: "daily", "weekly", "monthly"
	FetchIndexKlines(indexCode string, period string, limit int) ([]KlineData, error)
}

// SymbolInfo 交易对信息
type SymbolInfo struct {
	Code     string  `json:"code"`
	Name     string  `json:"name"`
	Type     string  `json:"type"` // spot, futures
	HotScore float64 `json:"hot_score"`
}

// KlineData K线数据
type KlineData struct {
	OpenTime    time.Time `json:"open_time"`
	CloseTime   time.Time `json:"close_time"`
	Open        float64   `json:"open"`
	High        float64   `json:"high"`
	Low         float64   `json:"low"`
	Close       float64   `json:"close"`
	Volume      float64   `json:"volume"`
	QuoteVolume float64   `json:"quote_volume"`
	TradesCount int       `json:"trades_count"`
}

// Ticker 实时行情
type Ticker struct {
	Symbol      string  `json:"symbol"`
	LastPrice   float64 `json:"last_price"`
	High24h     float64 `json:"high_24h"`
	Low24h      float64 `json:"low_24h"`
	Volume24h   float64 `json:"volume_24h"`
	QuoteVolume float64 `json:"quote_volume_24h"`
	PriceChange float64 `json:"price_change"`
	ChangePct   float64 `json:"change_pct"`
	Timestamp   int64   `json:"timestamp"`
}

// convertToModel 将KlineData转换为models.Kline
func convertToModel(symbolID int, period string, k KlineData) *models.Kline {
	return &models.Kline{
		SymbolID:    symbolID,
		Period:      period,
		OpenTime:    k.OpenTime,
		CloseTime:   k.CloseTime,
		OpenPrice:   k.Open,
		HighPrice:   k.High,
		LowPrice:    k.Low,
		ClosePrice:  k.Close,
		Volume:      k.Volume,
		QuoteVolume: k.QuoteVolume,
		TradesCount: k.TradesCount,
		IsClosed:    true,
		CreatedAt:   time.Now().UTC(),
	}
}
