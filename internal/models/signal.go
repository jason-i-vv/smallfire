package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

type JSONB map[string]interface{}

func (j JSONB) Value() (driver.Value, error) {
	return json.Marshal(j)
}

func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, j)
}

type Signal struct {
	ID               int        `json:"id" db:"id"`
	SymbolID         int        `json:"symbol_id" db:"symbol_id"`
	SymbolCode       string     `json:"symbol_code" db:"symbol_code"`
	SignalType       string     `json:"signal_type" db:"signal_type"`
	SourceType       string     `json:"source_type" db:"source_type"`
	Direction        string     `json:"direction" db:"direction"`
	Strength         int        `json:"strength" db:"strength"`
	Price            float64    `json:"price" db:"price"`
	TargetPrice      *float64   `json:"target_price,omitempty" db:"target_price"`
	StopLossPrice    *float64   `json:"stop_loss_price,omitempty" db:"stop_loss_price"`
	Period           string     `json:"period" db:"period"`
	SignalData       *JSONB     `json:"signal_data,omitempty" db:"signal_data"`
	Description      string     `json:"description" db:"description"`
	Status           string     `json:"status" db:"status"`
	ConfirmedAt      *time.Time `json:"confirmed_at,omitempty" db:"confirmed_at"`
	ExpiredAt        *time.Time `json:"expired_at,omitempty" db:"expired_at"`
	TriggeredAt      *time.Time `json:"triggered_at,omitempty" db:"triggered_at"`
	NotificationSent bool       `json:"notification_sent" db:"notification_sent"`
	CreatedAt        time.Time  `json:"created_at" db:"created_at"`
	KlineTime        *time.Time `json:"kline_time,omitempty" db:"kline_time"`
	// 评分相关
	Score           int        `json:"score" db:"score"`
	ScoreDetails    *JSONB     `json:"score_details,omitempty" db:"score_details"`
	ValidUntil      *time.Time `json:"valid_until,omitempty" db:"valid_until"`
	ConfluenceInfo  *JSONB     `json:"confluence_info,omitempty" db:"confluence_info"`
	// 关联的交易机会
	OpportunityID   *int       `json:"opportunity_id,omitempty" db:"-"`
}

// SignalQuery 信号查询参数
type SignalQuery struct {
	Market     string // 市场代码: bybit, a_stock, us_stock
	SymbolCode string // 标的代码: BTCUSDT 等
	SourceType string // 策略来源: box, trend, key_level, volume, wick
	SignalType string // 信号类型
	Direction  string // 方向: long, short
	Strength   *int   // 强度: 1, 2, 3
	Status     string // 状态: pending, active, triggered, expired
	StartDate  *time.Time
	EndDate    *time.Time
	Page       int
	PageSize   int
}

const (
	SignalTypeBoxBreakout       = "box_breakout"
	SignalTypeBoxBreakdown      = "box_breakdown"
	SignalTypeTrendReversal     = "trend_reversal"
	SignalTypeTrendRetracement  = "trend_retracement"
	SignalTypeResistanceBreak   = "resistance_break"
	SignalTypeSupportBreak     = "support_break"
	SignalTypeVolumeSurge       = "volume_surge"
	SignalTypePriceSurge        = "price_surge"
	SignalTypePriceSurgeUp      = "price_surge_up"
	SignalTypePriceSurgeDown    = "price_surge_down"

	// 上下引线信号类型
	SignalTypeUpperWickReversal = "upper_wick_reversal"  // 上引线反转（空头）
	SignalTypeLowerWickReversal = "lower_wick_reversal"  // 下引线反转（多头）
	SignalTypeFakeBreakoutUpper = "fake_breakout_upper"  // 假突破上引线（空头）
	SignalTypeFakeBreakoutLower = "fake_breakout_lower"  // 假突破下引线（多头）

	// K线形态信号类型
	SignalTypeEngulfingBullish = "engulfing_bullish" // 阳包阴（看多）
	SignalTypeEngulfingBearish = "engulfing_bearish" // 阴包阳（看空）
	SignalTypeMomentumBullish  = "momentum_bullish"  // 三连阳实体递增（看多）
	SignalTypeMomentumBearish  = "momentum_bearish"  // 三连阴实体递增（看空）
	SignalTypeMorningStar      = "morning_star"       // 早晨之星（看多）
	SignalTypeEveningStar      = "evening_star"       // 黄昏之星（看空）

	// MACD信号类型
	SignalTypeMACD = "macd"

	SourceTypeBox        = "box"
	SourceTypeTrend      = "trend"
	SourceTypeKeyLevel   = "key_level"
	SourceTypeVolume     = "volume"
	SourceTypeWick       = "wick"
	SourceTypeCandlestick = "candlestick"
	SourceTypeMACD       = "macd"

	DirectionLong  = "long"
	DirectionShort = "short"

	SignalStatusPending   = "pending"
	SignalStatusActive    = "active"
	SignalStatusTriggered = "triggered"
	SignalStatusExpired   = "expired"
	SignalStatusCancelled = "cancelled"
	SignalStatusAbsorbed  = "absorbed" // 被交易机会吸收
)
