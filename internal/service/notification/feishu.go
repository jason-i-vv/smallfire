package notification

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/internal/repository"
	"github.com/smallfire/starfire/pkg/utils"
)

// FeishuNotifier 飞书通知
type FeishuNotifier struct {
	config     *FeishuConfig
	httpClient *http.Client
	notifyRepo repository.NotificationRepo
}

// FeishuConfig 飞书配置
type FeishuConfig struct {
	Enabled         bool     `mapstructure:"enabled"`
	WebhookURL      string   `mapstructure:"webhook_url"`
	SendSummary     bool     `mapstructure:"send_summary"`
	SummaryInterval int      `mapstructure:"summary_interval"` // 小时
	SummaryTimes    []string `mapstructure:"summary_times"`    // ["09:00", "15:00", "21:00"]
}

func NewFeishuNotifier(cfg *FeishuConfig, notifyRepo repository.NotificationRepo) *FeishuNotifier {
	return &FeishuNotifier{
		config:     cfg,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		notifyRepo: notifyRepo,
	}
}

func (f *FeishuNotifier) Channel() string {
	return models.ChannelFeishu
}

func (f *FeishuNotifier) Send(content *NotifyContent) error {
	if !f.config.Enabled {
		return nil
	}

	// 构建飞书消息
	msg := f.buildMessage(content)

	// 保存通知记录
	notify := &models.Notification{
		Channel:     models.ChannelFeishu,
		Content:     msg,
		Status:      models.NotifyStatusPending,
		RetryCount:  0,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	f.notifyRepo.Create(notify)

	// 发送请求
	resp, err := f.httpClient.Post(f.config.WebhookURL, "application/json", bytes.NewReader(msg))
	if err != nil {
		return f.handleError(notify, err)
	}
	defer resp.Body.Close()

	// 解析响应
	var result FeishuResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return f.handleError(notify, err)
	}

	if result.Code != 0 {
		err := fmt.Errorf("feishu api error: %s", result.Msg)
		return f.handleError(notify, err)
	}

	// 更新状态
	now := time.Now()
	notify.Status = models.NotifyStatusSent
	notify.SentAt = &now
	f.notifyRepo.Update(notify)

	return nil
}

func (f *FeishuNotifier) handleError(notify *models.Notification, err error) error {
	utils.Error("send feishu notification failed", zap.Error(err))

	notify.Status = models.NotifyStatusFailed
	errMsg := err.Error()
	notify.ErrorMessage = &errMsg
	notify.RetryCount++
	f.notifyRepo.Update(notify)

	return err
}

func (f *FeishuNotifier) buildMessage(content *NotifyContent) json.RawMessage {
	var blocks []FeishuBlock

	// 标题
	blocks = append(blocks, FeishuBlock{
		Tag:     "markdown",
		Content: fmt.Sprintf("**%s**", content.Title),
	})

	// 分割线
	blocks = append(blocks, FeishuBlock{
		Tag: "hr",
	})

	// 消息内容
	blocks = append(blocks, FeishuBlock{
		Tag:     "markdown",
		Content: content.Message,
	})

	// 附加数据
	if len(content.Data) > 0 {
		for k, v := range content.Data {
			blocks = append(blocks, FeishuBlock{
				Tag:     "markdown",
				Content: fmt.Sprintf("**%s**: %v", k, v),
			})
		}
	}

	// 底部信息
	blocks = append(blocks, FeishuBlock{
		Tag:     "markdown",
		Content: fmt.Sprintf("\n> 🕐 %s", FormatCSTTime(time.Now())),
	})

	msg := map[string]interface{}{
		"msg_type": "interactive",
		"card": FeishuCard{
			Header: FeishuHeader{
				Title: FeishuHeaderTitle{
					Tag:  "plain_text",
					Text: content.Title,
				},
				Template: f.getTemplateByType(content.Type),
			},
			Elements: blocks,
		},
	}

	data, _ := json.Marshal(msg)
	return data
}

func (f *FeishuNotifier) getTemplateByType(msgType string) string {
	switch msgType {
	case "signal":
		return "purple"
	case "trade":
		return "green"
	case "alert":
		return "red"
	default:
		return "blue"
	}
}

func (f *FeishuNotifier) SendSignalNotification(signal *models.Signal) error {
	direction := "做多"
	emoji := "🟢"
	if signal.Direction == "short" {
		direction = "做空"
		emoji = "🔴"
	}

	strength := ""
	for i := 0; i < signal.Strength; i++ {
		strength += "⭐"
	}

	signalType := f.getSignalTypeName(signal.SignalType)

	content := &NotifyContent{
		Title:   fmt.Sprintf("%s %s %s", emoji, signal.SymbolCode, signalType),
		Type:    models.NotifyTypeSignal,
		Message: fmt.Sprintf("📊 **周期**: %s\n📈 **方向**: %s\n⭐ **强度**: %s\n💰 **信号价格**: %.4f",
			signal.Period, direction, strength, signal.Price),
	}

	return f.Send(content)
}

func (f *FeishuNotifier) getSignalTypeName(signalType string) string {
	names := map[string]string{
		"box_breakout":         "箱体向上突破",
		"box_breakdown":        "箱体向下突破",
		"trend_reversal":       "趋势反转",
		"trend_retracement":    "趋势回撤",
		"resistance_break":     "阻力位突破",
		"support_break":        "支撑位跌破",
		"volume_surge":         "成交量放大",
		"price_surge_up":       "价格急涨",
		"price_surge_down":     "价格急跌",
		"upper_wick_reversal":  "上引线反转",
		"lower_wick_reversal":  "下引线反转",
		"fake_breakout_upper":  "假突破上引线",
		"fake_breakout_lower":  "假突破下引线",
	}
	if name, ok := names[signalType]; ok {
		return name
	}
	return signalType
}

func (f *FeishuNotifier) SendTradeOpenedNotification(track *models.TradeTrack) error {
	direction := "做多"
	emoji := "🟢"
	if track.Direction == "short" {
		direction = "做空"
		emoji = "🔴"
	}

	content := &NotifyContent{
		Title:   fmt.Sprintf("%s %s 开仓", emoji, track.SymbolCode),
		Type:    models.NotifyTypeTrade,
		Message: fmt.Sprintf("📊 **方向**: %s\n💰 **入场价格**: %.4f\n📦 **仓位数量**: %.4f\n📐 **止损价格**: %.4f\n🎯 **止盈价格**: %.4f",
			direction, *track.EntryPrice, *track.Quantity, *track.StopLossPrice, *track.TakeProfitPrice),
		Data: map[string]interface{}{
			"symbol":      track.SymbolCode,
			"direction":   track.Direction,
			"entry_price": *track.EntryPrice,
			"quantity":    *track.Quantity,
			"stop_loss":   *track.StopLossPrice,
			"take_profit": *track.TakeProfitPrice,
		},
	}

	return f.Send(content)
}

func (f *FeishuNotifier) SendTradeClosedNotification(track *models.TradeTrack) error {
	emoji := "❌"
	if track.PnL != nil && *track.PnL > 0 {
		emoji = "✅"
	}

	exitReason := f.getExitReasonName(*track.ExitReason)

	pnlStr := ""
	if track.PnL != nil {
		if *track.PnL > 0 {
			pnlStr = fmt.Sprintf("+%.2f", *track.PnL)
		} else {
			pnlStr = fmt.Sprintf("%.2f", *track.PnL)
		}
	}

	content := &NotifyContent{
		Title:   fmt.Sprintf("%s %s 平仓 %s", emoji, track.SymbolCode, pnlStr),
		Type:    models.NotifyTypeTrade,
		Message: fmt.Sprintf("📊 **平仓原因**: %s\n💰 **出场价格**: %.4f\n💵 **盈亏**: %s (%.2f%%)\n⏱ **持仓时间**: %s",
			exitReason, *track.ExitPrice, pnlStr, *track.PnLPercent*100,
			FormatHoldingTime(track.EntryTime, track.ExitTime)),
	}

	return f.Send(content)
}

func (f *FeishuNotifier) getExitReasonName(reason string) string {
	names := map[string]string{
		"stop_loss":     "止损",
		"take_profit":   "止盈",
		"trailing_stop": "移动止损",
		"manual":        "手动平仓",
		"expired":       "信号过期",
	}
	if name, ok := names[reason]; ok {
		return name
	}
	return reason
}

func (f *FeishuNotifier) SendSummaryNotification(stats *SummaryStats) error {
	content := &NotifyContent{
		Title:   "📊 星火量化 - 交易汇总",
		Type:    models.NotifyTypeSummary,
		Message: fmt.Sprintf("📈 **今日统计**\n├ 交易次数: %d\n├ 盈利次数: %d\n├ 亏损次数: %d\n├ 胜率: %.1f%%\n├ 总盈亏: %s\n└ 最大回撤: %.2f%%\n\n💰 **账户状态**\n├ 初始资金: %.2f\n├ 当前权益: %.2f\n├ 总收益率: %.2f%%\n\n📋 **信号统计**\n├ 今日信号: %d\n└ 活跃持仓: %d",
			stats.TodayTrades, stats.WinTrades, stats.LossTrades, stats.WinRate*100,
			FormatPnL(stats.TotalPnL), stats.MaxDrawdownPct*100,
			stats.InitialCapital, stats.CurrentCapital, stats.TotalReturn*100,
			stats.TodaySignals, stats.OpenPositions),
	}

	return f.Send(content)
}
