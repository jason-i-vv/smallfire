package models

import "time"

type Kline struct {
	ID          int64     `json:"id" db:"id"`
	SymbolID    int       `json:"symbol_id" db:"symbol_id"`
	Period      string    `json:"period" db:"period"`
	OpenTime    time.Time `json:"open_time" db:"open_time"`
	CloseTime   time.Time `json:"close_time" db:"close_time"`
	OpenPrice   float64   `json:"open_price" db:"open_price"`
	HighPrice   float64   `json:"high_price" db:"high_price"`
	LowPrice    float64   `json:"low_price" db:"low_price"`
	ClosePrice  float64   `json:"close_price" db:"close_price"`
	Volume      float64   `json:"volume" db:"volume"`
	QuoteVolume float64   `json:"quote_volume" db:"quote_volume"`
	TradesCount int       `json:"trades_count" db:"trades_count"`
	IsClosed    bool      `json:"is_closed" db:"is_closed"`
	EMAShort    *float64  `json:"ema_short,omitempty" db:"ema_short"`
	EMAMedium   *float64  `json:"ema_medium,omitempty" db:"ema_medium"`
	EMALong     *float64  `json:"ema_long,omitempty" db:"ema_long"`
	MACD        *float64  `json:"macd,omitempty" db:"macd"`
	MACDSignal  *float64  `json:"macd_signal,omitempty" db:"macd_signal"`
	MACDHist    *float64  `json:"macd_hist,omitempty" db:"macd_hist"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

const (
	Period15m = "15m"
	Period1h  = "1h"
	Period1d  = "1d"
)
