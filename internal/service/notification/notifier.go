package notification

import (
	"encoding/json"
	"time"

	"github.com/smallfire/starfire/internal/models"
)

// Notifier 通知接口
type Notifier interface {
	// Send 发送通知
	Send(content *NotifyContent) error

	// Channel 获取频道名称
	Channel() string

	// SendSignalNotification 发送信号通知
	SendSignalNotification(signal *models.Signal) error

	// SendOpportunityNotification 发送交易机会通知
	SendOpportunityNotification(opp *models.TradingOpportunity) error

	// SendTradeOpenedNotification 发送开仓通知
	SendTradeOpenedNotification(track *models.TradeTrack) error

	// SendTradeClosedNotification 发送平仓通知
	SendTradeClosedNotification(track *models.TradeTrack) error

	// SendSummaryNotification 发送汇总通知
	SendSummaryNotification(stats *SummaryStats) error
}

// NotifyContent 通知内容
type NotifyContent struct {
	Title   string                 `json:"title"`
	Message string                 `json:"message"`
	Type    string                 `json:"type"` // signal, trade, alert, summary
	Data    map[string]interface{} `json:"data"`
}

// SummaryStats 汇总统计
type SummaryStats struct {
	TodayTrades    int
	WinTrades      int
	LossTrades     int
	WinRate        float64
	TotalPnL       float64
	MaxDrawdownPct float64
	InitialCapital float64
	CurrentCapital float64
	TotalReturn    float64
	TodaySignals   int
	OpenPositions  int
}

// FeishuBlock 飞书卡片块
type FeishuBlock struct {
	Tag     string `json:"tag,omitempty"`
	Content string `json:"content,omitempty"`
}

// FeishuCard 飞书卡片
type FeishuCard struct {
	Header   FeishuHeader `json:"header"`
	Elements []FeishuBlock `json:"elements"`
}

// FeishuHeader 飞书卡片头部
type FeishuHeader struct {
	Title    FeishuHeaderTitle `json:"title"`
	Template string             `json:"template"`
}

// FeishuHeaderTitle 飞书卡片标题
type FeishuHeaderTitle struct {
	Tag  string `json:"tag"`
	Text string `json:"text"`
}

// FeishuResponse 飞书响应
type FeishuResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// 中国时区
var cstZone = time.FixedZone("CST", 8*3600)

// GetCSTTime 获取中国时区时间
func GetCSTTime() time.Time {
	return time.Now().In(cstZone)
}

// FormatCSTTime 格式化中国时区时间
func FormatCSTTime(t time.Time) string {
	return t.In(cstZone).Format("2006-01-02 15:04:05")
}

// FormatHoldingTime 格式化持仓时间
func FormatHoldingTime(entryTime, exitTime *time.Time) string {
	if entryTime == nil || exitTime == nil {
		return "-"
	}
	duration := exitTime.Sub(*entryTime)
	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60
	if hours > 0 {
		return string(rune(hours)) + "小时" + string(rune(minutes)) + "分钟"
	}
	return string(rune(minutes)) + "分钟"
}

// FormatPnL 格式化盈亏
func FormatPnL(pnl float64) string {
	if pnl > 0 {
		return "+" + string(rune(int64(pnl)))
	}
	return string(rune(int64(pnl)))
}

// JSONMarshal 安全JSON序列化
func JSONMarshal(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("{}")
	}
	return data
}
