# 需求文档：通知模块

**需求编号**: REQ-NOTIFY-001
**模块**: 通知
**优先级**: P0
**状态**: 待开发
**前置依赖**: REQ-INF-001 (基础设施)
**创建时间**: 2024-03-22

---

## 1. 需求概述

实现消息通知模块，负责：
- 飞书群聊通知
- 交易信号推送
- 定时汇总推送
- 通知记录与重试

---

## 2. 数据模型

### 2.1 Notification 模型

```go
// internal/models/notification.go
type Notification struct {
    ID          int64           `json:"id"`
    SignalID    *int64          `json:"signal_id"`
    Channel     string          `json:"channel"`     // feishu, email, sms
    Content     json.RawMessage `json:"content"`    // 消息内容(JSON)
    Status      string          `json:"status"`     // pending, sent, failed
    SentAt      *time.Time     `json:"sent_at"`
    ErrorMsg    *string         `json:"error_message"`
    RetryCount  int             `json:"retry_count"`
    CreatedAt   time.Time      `json:"created_at"`
}

// Channel 常量
const (
    ChannelFeishu = "feishu"
    ChannelEmail = "email"
    ChannelSMS   = "sms"
)

// Status 常量
const (
    NotifyStatusPending = "pending"
    NotifyStatusSent   = "sent"
    NotifyStatusFailed = "failed"
)
```

---

## 3. 通知接口

### 3.1 Notifier 接口

```go
// internal/service/notification/notifier.go
type Notifier interface {
    // 发送通知
    Send(content *NotifyContent) error

    // 获取频道名称
    Channel() string
}

// NotifyContent 通知内容
type NotifyContent struct {
    Title   string                 `json:"title"`
    Message string                 `json:"message"`
    Type    string                 `json:"type"`   // signal, trade, alert, summary
    Data    map[string]interface{} `json:"data"`
}
```

---

## 4. 飞书通知实现

### 4.1 FeishuNotifier

```go
// internal/service/notification/feishu.go
type FeishuNotifier struct {
    config     *FeishuConfig
    httpClient *http.Client
    notifyRepo repository.NotificationRepo
}

type FeishuConfig struct {
    Enabled        bool
    WebhookURL     string
    SendSummary    bool           `yaml:"send_summary"`
    SummaryInterval int           `yaml:"summary_interval"` // 小时
    SummaryTimes   []string      `yaml:"summary_times"`   // ["09:00", "15:00", "21:00"]
}

func NewFeishuNotifier(cfg *FeishuConfig, notifyRepo repository.NotificationRepo) *FeishuNotifier {
    return &FeishuNotifier{
        config:     cfg,
        httpClient: &http.Client{Timeout: 10 * time.Second},
        notifyRepo: notifyRepo,
    }
}

func (f *FeishuNotifier) Channel() string {
    return ChannelFeishu
}
```

### 4.2 发送消息

```go
func (f *FeishuNotifier) Send(content *NotifyContent) error {
    if !f.config.Enabled {
        return nil
    }

    // 构建飞书消息
    msg := f.buildMessage(content)

    // 保存通知记录
    notify := &model.Notification{
        Channel:   ChannelFeishu,
        Content:   msg,
        Status:    NotifyStatusPending,
        RetryCount: 0,
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
    notify.Status = NotifyStatusSent
    notify.SentAt = &now
    f.notifyRepo.Update(notify)

    return nil
}

type FeishuResponse struct {
    Code int    `json:"code"`
    Msg  string `json:"msg"`
}
```

### 4.3 消息构建

```go
func (f *FeishuNotifier) buildMessage(content *NotifyContent) json.RawMessage {
    var blocks []FeishuBlock

    // 标题
    blocks = append(blocks, FeishuBlock{
        Tag:       "markdown",
        Content:   fmt.Sprintf("**%s**", content.Title),
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
        blocks = append(blocks, f.buildDataBlocks(content)...)
    }

    // 底部信息
    blocks = append(blocks, FeishuBlock{
        Tag:     "markdown",
        Content: fmt.Sprintf("\n> 🕐 %s", time.Now().In(time.FixedZone("CST", 8*3600)).Format("2006-01-02 15:04:05")),
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

type FeishuBlock struct {
    Tag     string `json:"tag,omitempty"`
    Content string `json:"content,omitempty"`
}

type FeishuCard struct {
    Header  FeishuHeader `json:"header"`
    Elements []FeishuBlock `json:"elements"`
}

type FeishuHeader struct {
    Title   FeishuHeaderTitle `json:"title"`
    Template string           `json:"template"`
}

type FeishuHeaderTitle struct {
    Tag  string `json:"tag"`
    Text string `json:"text"`
}
```

---

## 5. 通知类型

### 5.1 信号通知

```go
// SendSignalNotification 发送信号通知
func (n *FeishuNotifier) SendSignalNotification(signal *model.Signal) error {
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

    signalType := n.getSignalTypeName(signal.SignalType)

    content := &NotifyContent{
        Title:  fmt.Sprintf("%s %s信号", emoji, signal.SymbolCode),
        Type:   "signal",
        Message: fmt.Sprintf(`📊 **信号类型**: %s
📈 **方向**: %s
⭐ **强度**: %s
💰 **信号价格**: %.4f`, signalType, direction, strength, signal.Price),
        Data: map[string]interface{}{
            "symbol":      signal.SymbolCode,
            "signal_type": signal.SignalType,
            "direction":   signal.Direction,
            "strength":    signal.Strength,
            "price":       signal.Price,
        },
    }

    return n.Send(content)
}

func (n *FeishuNotifier) getSignalTypeName(signalType string) string {
    names := map[string]string{
        "box_breakout":        "箱体向上突破",
        "box_breakdown":       "箱体向下突破",
        "trend_reversal":      "趋势反转",
        "trend_retracement":   "趋势回撤",
        "resistance_break":    "阻力位突破",
        "support_break":      "支撑位跌破",
        "volume_surge":       "成交量放大",
        "price_surge_up":     "价格急涨",
        "price_surge_down":   "价格急跌",
    }
    if name, ok := names[signalType]; ok {
        return name
    }
    return signalType
}
```

### 5.2 交易通知

```go
// SendTradeOpenedNotification 发送开仓通知
func (n *FeishuNotifier) SendTradeOpenedNotification(track *model.TradeTrack) error {
    direction := "做多"
    emoji := "🟢"
    if track.Direction == "short" {
        direction = "做空"
        emoji = "🔴"
    }

    content := &NotifyContent{
        Title:  fmt.Sprintf("%s %s 开仓", emoji, track.SymbolCode),
        Type:   "trade",
        Message: fmt.Sprintf(`📊 **方向**: %s
💰 **入场价格**: %.4f
📦 **仓位数量**: %.4f
📐 **止损价格**: %.4f
🎯 **止盈价格**: %.4f`,
            direction,
            *track.EntryPrice,
            *track.Quantity,
            *track.StopLossPrice,
            *track.TakeProfitPrice,
        ),
        Data: map[string]interface{}{
            "symbol":       track.SymbolCode,
            "direction":    track.Direction,
            "entry_price":  *track.EntryPrice,
            "quantity":     *track.Quantity,
            "stop_loss":    *track.StopLossPrice,
            "take_profit":  *track.TakeProfitPrice,
        },
    }

    return n.Send(content)
}

// SendTradeClosedNotification 发送平仓通知
func (n *FeishuNotifier) SendTradeClosedNotification(track *model.TradeTrack) error {
    emoji := "❌"
    if track.PnL != nil && *track.PnL > 0 {
        emoji = "✅"
    }

    exitReason := n.getExitReasonName(*track.ExitReason)

    pnlStr := ""
    if track.PnL != nil {
        if *track.PnL > 0 {
            pnlStr = fmt.Sprintf("+%.2f", *track.PnL)
        } else {
            pnlStr = fmt.Sprintf("%.2f", *track.PnL)
        }
    }

    content := &NotifyContent{
        Title:  fmt.Sprintf("%s %s 平仓 %s", emoji, track.SymbolCode, pnlStr),
        Type:   "trade",
        Message: fmt.Sprintf(`📊 **平仓原因**: %s
💰 **出场价格**: %.4f
💵 **盈亏**: %s (%.2f%%)
⏱ **持仓时间**: %s`,
            exitReason,
            *track.ExitPrice,
            pnlStr,
            *track.PnLPercent*100,
            formatHoldingTime(track.EntryTime, track.ExitTime),
        ),
    }

    return n.Send(content)
}

func (n *FeishuNotifier) getExitReasonName(reason string) string {
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
```

### 5.3 汇总通知

```go
// SendSummaryNotification 发送汇总通知
func (n *FeishuNotifier) SendSummaryNotification(stats *SummaryStats) error {
    content := &NotifyContent{
        Title:  "📊 星火量化 - 交易汇总",
        Type:   "summary",
        Message: fmt.Sprintf(`📈 **今日统计**
├ 交易次数: %d
├ 盈利次数: %d
├ 亏损次数: %d
├ 胜率: %.1f%%
├ 总盈亏: %s
└ 最大回撤: %.2f%%

💰 **账户状态**
├ 初始资金: %.2f
├ 当前权益: %.2f
├ 总收益率: %.2f%%

📋 **信号统计**
├ 今日信号: %d
├ 活跃持仓: %d`,
            stats.TodayTrades,
            stats.WinTrades,
            stats.LossTrades,
            stats.WinRate*100,
            formatPnL(stats.TotalPnL),
            stats.MaxDrawdownPct*100,
            stats.InitialCapital,
            stats.CurrentCapital,
            stats.TotalReturn*100,
            stats.TodaySignals,
            stats.OpenPositions,
        ),
    }

    return n.Send(content)
}

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
```

---

## 6. 定时汇总服务

### 6.1 SummaryService

```go
// internal/service/notification/summary_service.go
type SummaryService struct {
    notifier    *FeishuNotifier
    statsService *trading.StatisticsService
    config      *FeishuConfig
    stopCh      chan struct{}
    wg          sync.WaitGroup
}

func (s *SummaryService) Start() {
    s.stopCh = make(chan struct{})

    // 解析汇总时间
    times := s.parseSummaryTimes(s.config.SummaryTimes)

    s.wg.Add(1)
    go s.runLoop(times)
}

func (s *SummaryService) Stop() {
    close(s.stopCh)
    s.wg.Wait()
}

func (s *SummaryService) runLoop(times []string) {
    defer s.wg.Done()

    for {
        select {
        case <-s.stopCh:
            return
        default:
            s.waitUntilNextSummary(times)
            s.sendSummary()
        }
    }
}

func (s *SummaryService) sendSummary() {
    stats, err := s.getTodayStats()
    if err != nil {
        log.Errorf("get stats failed: %v", err)
        return
    }

    if err := s.notifier.SendSummaryNotification(stats); err != nil {
        log.Errorf("send summary failed: %v", err)
    }
}

func (s *SummaryService) getTodayStats() (*SummaryStats, error) {
    // 获取今日统计数据
    now := time.Now().In(time.FixedZone("CST", 8*3600))
    today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.FixedZone("CST", 8*3600))

    tradeStats, _ := s.statsService.GetStatistics(&today, nil)

    stats := &SummaryStats{
        TodayTrades:    tradeStats.TotalTrades,
        WinTrades:      tradeStats.WinTrades,
        LossTrades:     tradeStats.LossTrades,
        WinRate:        tradeStats.WinRate,
        TotalPnL:       tradeStats.TotalPnL,
        MaxDrawdownPct: tradeStats.MaxDrawdownPct,
        InitialCapital: tradeStats.InitialCapital,
        CurrentCapital: tradeStats.CurrentCapital,
        TotalReturn:    tradeStats.TotalReturn,
    }

    return stats, nil
}
```

---

## 7. 通知管理器

### 7.1 Manager 实现

```go
// internal/service/notification/manager.go
type Manager struct {
    notifiers    map[string]Notifier
    summarySvc   *SummaryService
    notifyRepo   repository.NotificationRepo
}

func NewManager(notifiers []Notifier, summarySvc *SummaryService, notifyRepo repository.NotificationRepo) *Manager {
    m := &Manager{
        notifiers:  make(map[string]Notifier),
        summarySvc: summarySvc,
        notifyRepo: notifyRepo,
    }

    for _, n := range notifiers {
        m.notifiers[n.Channel()] = n
    }

    return m
}

// SendToAll 发送到所有渠道
func (m *Manager) SendToAll(content *NotifyContent) {
    for _, notifier := range m.notifiers {
        go func(n Notifier) {
            if err := n.Send(content); err != nil {
                log.Errorf("send notification failed: %v", err)
            }
        }(notifier)
    }
}

// SendSignal 发送信号通知
func (m *Manager) SendSignal(signal *model.Signal) {
    if feishu, ok := m.notifiers[ChannelFeishu]; ok {
        go func() {
            if err := feishu.Send(&NotifyContent{
                Title:  signal.SymbolCode + " 信号",
                Type:   "signal",
                Message: formatSignalMessage(signal),
                Data:   formatSignalData(signal),
            }); err != nil {
                log.Errorf("send signal notification failed: %v", err)
            }
        }()
    }
}

// SendTradeOpened 发送开仓通知
func (m *Manager) SendTradeOpened(track *model.TradeTrack) {
    if feishu, ok := m.notifiers[ChannelFeishu]; ok {
        go func() {
            if err := feishu.SendTradeOpenedNotification(track); err != nil {
                log.Errorf("send trade opened notification failed: %v", err)
            }
        }()
    }
}

// SendTradeClosed 发送平仓通知
func (m *Manager) SendTradeClosed(track *model.TradeTrack) {
    if feishu, ok := m.notifiers[ChannelFeishu]; ok {
        go func() {
            if err := feishu.SendTradeClosedNotification(track); err != nil {
                log.Errorf("send trade closed notification failed: %v", err)
            }
        }()
    }
}
```

---

## 8. 配置文件

```yaml
# config.yml
feishu:
  enabled: true
  webhook_url: "https://open.feishu.cn/open-apis/bot/v2/hook/c585be48-9114-4f71-bf3d-0f82f98ba4d6"
  send_summary: true
  summary_interval: 6      # 每6小时发送一次汇总
  summary_times:           # 指定时间发送
    - "09:00"
    - "15:00"
    - "21:00"
```

---

## 9. 文件结构

```
internal/service/notification/
├── notifier.go          # 通知接口
├── feishu.go            # 飞书通知实现
├── manager.go           # 通知管理器
├── summary_service.go    # 汇总服务
└── format.go           # 格式化工具
```

---

## 10. 验收标准

### 10.1 功能验收

- [ ] 飞书webhook能正常发送消息
- [ ] 信号通知格式正确
- [ ] 交易通知（开仓/平仓）格式正确
- [ ] 定时汇总能正常发送
- [ ] 通知记录正确保存

### 10.2 消息格式验收

- [ ] 卡片样式正确
- [ ] 颜色区分正确（信号紫色、交易绿色、警告红色）
- [ ] 时间显示使用 UTC+8 时区

---

## 11. 注意事项

1. **异步发送**：通知发送使用异步方式，不阻塞主流程
2. **错误处理**：发送失败需要记录错误信息
3. **限流处理**：飞书有发送频率限制，需要处理限流
4. **时间格式**：所有时间统一使用 UTC+8 时区

---

**前置依赖**: REQ-INF-001
**执行人**: 待分配
**预计工时**: 3小时
**实际完成时间**: 待填写
