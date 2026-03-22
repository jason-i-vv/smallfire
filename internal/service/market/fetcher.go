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

	// 获取K线数据
	FetchKlines(symbol string, period string, limit int) ([]KlineData, error)

	// 获取实时价格
	FetchTicker(symbol string) (*Ticker, error)
}

// SymbolInfo 交易对信息
type SymbolInfo struct {
	Code     string  `json:"code"`
	Name     string  `json:"name"`
	Type     string  `json:"type"`    // spot, futures
	HotScore float64 `json:"hot_score"`
}

// KlineData K线数据
type KlineData struct {
	OpenTime     time.Time `json:"open_time"`
	CloseTime    time.Time `json:"close_time"`
	Open         float64   `json:"open"`
	High         float64   `json:"high"`
	Low          float64   `json:"low"`
	Close        float64   `json:"close"`
	Volume       float64   `json:"volume"`
	QuoteVolume  float64   `json:"quote_volume"`
	TradesCount  int       `json:"trades_count"`
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
		SymbolID:     symbolID,
		Period:       period,
		OpenTime:     k.OpenTime,
		CloseTime:    k.CloseTime,
		OpenPrice:    k.Open,
		HighPrice:    k.High,
		LowPrice:     k.Low,
		ClosePrice:   k.Close,
		Volume:       k.Volume,
		QuoteVolume:  k.QuoteVolume,
		TradesCount:  k.TradesCount,
		IsClosed:     true,
		CreatedAt:    time.Now(),
	}
}
