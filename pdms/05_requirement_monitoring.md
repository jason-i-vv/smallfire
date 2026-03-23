# 需求文档：实时监测模块

**需求编号**: REQ-MONITOR-001
**模块**: 实时监测
**优先级**: P0
**状态**: 已完成
**前置依赖**: REQ-MARKET-001 (行情抓取)
**创建时间**: 2024-03-22

---

## 1. 需求概述

实现实时价格监测模块，负责：
- 管理多个标的的价格订阅
- 监测价格达到目标时触发事件
- 订阅计数管理（多个策略共享监测器）
- 服务重启后恢复监测器状态

### 1.2 核心功能

```
策略请求监测 → 创建/复用监测器 → 价格更新 → 判断触发条件 → 发送事件 → 策略处理
```

---

## 2. 数据模型

### 2.1 Monitoring 模型

```go
// internal/models/monitoring.go
type Monitoring struct {
    ID              int64      `json:"id"`
    SymbolID        int64      `json:"symbol_id"`
    SymbolCode      string     `json:"symbol_code"`      // 关联字段
    MonitorType     string     `json:"monitor_type"`   // price, box, trend
    TargetPrice     *float64   `json:"target_price"`   // 目标价格
    ConditionType   string     `json:"condition_type"`  // greater, less, cross_up, cross_down
    ReferencePrice  *float64   `json:"reference_price"`// 参考价格（用于cross条件）
    SubscriberCount int        `json:"subscriber_count"` // 订阅数量
    IsActive        bool       `json:"is_active"`
    TriggeredAt     *time.Time `json:"triggered_at"`
    CreatedAt       time.Time  `json:"created_at"`
    UpdatedAt       time.Time  `json:"updated_at"`
}

// MonitorType 常量
const (
    MonitorTypePrice = "price"
    MonitorTypeBox   = "box"
    MonitorTypeTrend = "trend"
)

// ConditionType 常量
const (
    ConditionGreater  = "greater"   // 价格大于目标
    ConditionLess    = "less"      // 价格小于目标
    ConditionCrossUp = "cross_up"   // 价格从下往上穿过
    ConditionCrossDown = "cross_down" // 价格从上往下穿过
)
```

---

## 3. 监测器接口

### 3.1 Monitor 接口

```go
// internal/service/monitoring/monitor.go
type Monitor interface {
    // 获取监测器ID
    ID() int64

    // 获取标的ID
    SymbolID() int64

    // 获取监测器类型
    Type() string

    // 检查是否触发
    Check(currentPrice float64, prevPrice float64) bool

    // 获取触发价格
    GetTargetPrice() float64

    // 获取订阅者信息
    GetSubscribers() []Subscriber

    // 是否仍在活跃
    IsActive() bool
}

// Subscriber 订阅者信息
type Subscriber struct {
    Type     string    // box, trade_track
    ID       int64     // 关联ID
    Callback func(Event) // 回调函数
}

// MonitorEvent 监测事件
type MonitorEvent struct {
    MonitorID    int64
    SymbolID     int64
    SymbolCode   string
    EventType    string
    TargetPrice  float64
    CurrentPrice float64
    TriggerTime  time.Time
}
```

---

## 4. 监测器工厂

### 4.1 Factory 实现

```go
// internal/service/monitoring/factory.go
type Factory struct {
    monitors    map[int64]Monitor  // symbolID -> Monitor
    tickerRepo  repository.TickerRepo
    eventChan   chan MonitorEvent
    mu          sync.RWMutex
    maxMonitors int
}

func NewFactory(tickerRepo repository.TickerRepo, maxMonitors int) *Factory {
    f := &Factory{
        monitors:   make(map[int64]Monitor),
        tickerRepo: tickerRepo,
        eventChan:  make(chan MonitorEvent, 1000),
        maxMonitors: maxMonitors,
    }
    return f
}
```

### 4.2 订阅管理

```go
// Subscribe 订阅价格监测
func (f *Factory) Subscribe(symbolID int64, targetPrice float64, condition string, subscriber Subscriber) error {
    f.mu.Lock()
    defer f.mu.Unlock()

    monitor, exists := f.monitors[symbolID]
    if !exists {
        // 检查最大数量
        if len(f.monitors) >= f.maxMonitors {
            return fmt.Errorf("已达到最大监测数量限制")
        }

        // 创建新监测器
        monitor = NewPriceMonitor(symbolID, f.tickerRepo, f.eventChan)
        f.monitors[symbolID] = monitor
    }

    // 添加订阅者
    monitor.AddSubscriber(subscriber)

    // 更新数据库
    f.saveMonitor(symbolID, targetPrice, condition)

    return nil
}

// Unsubscribe 取消订阅
func (f *Factory) Unsubscribe(symbolID int64, subscriberType string, subscriberID int64) error {
    f.mu.Lock()
    defer f.mu.Unlock()

    monitor, exists := f.monitors[symbolID]
    if !exists {
        return nil
    }

    // 移除订阅者
    monitor.RemoveSubscriber(subscriberType, subscriberID)

    // 如果没有订阅者了，销毁监测器
    if monitor.SubscriberCount() == 0 {
        delete(f.monitors, symbolID)
        f.deleteMonitor(symbolID)
    }

    return nil
}
```

---

## 5. 价格监测器

### 5.1 PriceMonitor 实现

```go
// internal/service/monitoring/price_monitor.go
type PriceMonitor struct {
    id            int64
    symbolID      int64
    symbolCode    string
    targetPrice   float64
    condition     string
    referencePrice float64  // 用于cross条件的前一个价格
    subscribers   []Subscriber
    isActive      bool
    createdAt     time.Time
    mu            sync.RWMutex
}

func NewPriceMonitor(symbolID int64, tickerRepo repository.TickerRepo, eventChan chan MonitorEvent) *PriceMonitor {
    return &PriceMonitor{
        id:          atomic.AddInt64(&monitorIDCounter, 1),
        symbolID:    symbolID,
        isActive:    true,
        createdAt:   time.Now(),
        eventChan:   eventChan,
    }
}

func (m *PriceMonitor) AddSubscriber(sub Subscriber) {
    m.mu.Lock()
    defer m.mu.Unlock()

    // 检查是否已存在
    for _, s := range m.subscribers {
        if s.Type == sub.Type && s.ID == sub.ID {
            return
        }
    }

    m.subscribers = append(m.subscribers, sub)
}

func (m *PriceMonitor) RemoveSubscriber(subType string, subID int64) {
    m.mu.Lock()
    defer m.mu.Unlock()

    for i, s := range m.subscribers {
        if s.Type == subType && s.ID == subID {
            m.subscribers = append(m.subscribers[:i], m.subscribers[i+1:]...)
            return
        }
    }
}

func (m *PriceMonitor) SubscriberCount() int {
    m.mu.RLock()
    defer m.mu.RUnlock()
    return len(m.subscribers)
}
```

### 5.2 触发检查

```go
// Check 检查是否触发
func (m *PriceMonitor) Check(currentPrice, prevPrice float64) bool {
    m.mu.Lock()
    defer m.mu.Unlock()

    if !m.isActive {
        return false
    }

    var triggered bool

    switch m.condition {
    case ConditionGreater:
        triggered = currentPrice > m.targetPrice

    case ConditionLess:
        triggered = currentPrice < m.targetPrice

    case ConditionCrossUp:
        triggered = prevPrice <= m.targetPrice && currentPrice > m.targetPrice

    case ConditionCrossDown:
        triggered = prevPrice >= m.targetPrice && currentPrice < m.targetPrice
    }

    if triggered {
        m.isActive = false
        // 发送事件
        m.emitEvent(currentPrice)
    }

    return triggered
}

func (m *PriceMonitor) emitEvent(currentPrice float64) {
    event := MonitorEvent{
        MonitorID:    m.id,
        SymbolID:     m.symbolID,
        SymbolCode:   m.symbolCode,
        EventType:    m.condition,
        TargetPrice:  m.targetPrice,
        CurrentPrice: currentPrice,
        TriggerTime:  time.Now(),
    }

    select {
    case m.eventChan <- event:
    default:
        log.Warn("monitor event channel is full")
    }

    // 通知所有订阅者
    for _, sub := range m.subscribers {
        go sub.Callback(event)
    }
}
```

---

## 6. 监测服务

### 6.1 Service 实现

```go
// internal/service/monitoring/service.go
type Service struct {
    factory    *Factory
    tickerRepo repository.TickerRepo
    interval   time.Duration
    stopCh     chan struct{}
    wg         sync.WaitGroup
}

func NewService(factory *Factory, tickerRepo repository.TickerRepo, interval time.Duration) *Service {
    return &Service{
        factory:    factory,
        tickerRepo: tickerRepo,
        interval:   interval,
    }
}

func (s *Service) Start() {
    s.stopCh = make(chan struct{})

    // 启动事件处理循环
    s.wg.Add(1)
    go s.eventLoop()

    // 启动价格检查循环
    s.wg.Add(1)
    go s.checkLoop()

    // 恢复活跃监测器
    s.recoverActiveMonitors()
}

func (s *Service) Stop() {
    close(s.stopCh)
    s.wg.Wait()
}
```

### 6.2 价格检查循环

```go
func (s *Service) checkLoop() {
    defer s.wg.Done()

    ticker := time.NewTicker(s.interval)
    defer ticker.Stop()

    for {
        select {
        case <-s.stopCh:
            return
        case <-ticker.C:
            s.checkAllMonitors()
        }
    }
}

func (s *Service) checkAllMonitors() {
    monitors := s.factory.GetActiveMonitors()

    for _, monitor := range monitors {
        symbolID := monitor.SymbolID()

        // 获取当前价格和前一个价格
        currentPrice := s.tickerRepo.GetPrice(symbolID)
        prevPrice := s.tickerRepo.GetPrevPrice(symbolID)

        if currentPrice == 0 {
            continue
        }

        // 检查是否触发
        if monitor.Check(currentPrice, prevPrice) {
            log.Infof("monitor triggered: symbol=%d, target=%f, current=%f",
                symbolID, monitor.GetTargetPrice(), currentPrice)
        }
    }
}
```

### 6.3 事件处理循环

```go
func (s *Service) eventLoop() {
    defer s.wg.Done()

    for {
        select {
        case <-s.stopCh:
            return
        case event := <-s.factory.EventChan():
            s.handleEvent(event)
        }
    }
}

func (s *Service) handleEvent(event monitoring.MonitorEvent) {
    // 根据事件类型分发处理
    switch event.EventType {
    case monitoring.ConditionCrossUp, monitoring.ConditionCrossDown:
        // 处理突破事件
        s.handleBreakoutEvent(event)
    case monitoring.ConditionGreater, monitoring.ConditionLess:
        // 处理价格到达事件
        s.handlePriceReachedEvent(event)
    }
}

func (s *Service) handleBreakoutEvent(event monitoring.MonitorEvent) {
    // 获取订阅该监测器的所有订阅者类型
    // 根据类型调用不同的处理逻辑
    log.Infof("breakout event: symbol=%s, price=%f", event.SymbolCode, event.CurrentPrice)
}
```

### 6.4 重启恢复

```go
func (s *Service) recoverActiveMonitors() {
    // 从数据库恢复活跃监测器
    monitors, err := s.monitorRepo.GetActiveMonitors()
    if err != nil {
        log.Errorf("recover monitors failed: %v", err)
        return
    }

    for _, m := range monitors {
        // 重新创建监测器
        s.factory.RecreateMonitor(m)
    }

    log.Infof("recovered %d active monitors", len(monitors))
}
```

---

## 7. WebSocket 推送

### 7.1 实时价格推送

```go
// internal/websocket/hub.go
type Hub struct {
    clients     map[*Client]bool
    subscribe   chan Subscription
    unsubscribe chan *Client
    broadcast   chan Message
    mu          sync.RWMutex
}

type Subscription struct {
    Client    *Client
    SymbolIDs []int64
}

func (h *Hub) SubscribePrice(symbolID int64) {
    // 订阅后开始推送该标的的实时价格
}

func (h *Hub) BroadcastPrice(symbolID int64, price *PriceUpdate) {
    h.mu.RLock()
    defer h.mu.RUnlock()

    for client := range h.clients {
        if client.IsSubscribed(symbolID) {
            client.Send <- price
        }
    }
}

// PriceUpdate 价格更新消息
type PriceUpdate struct {
    SymbolCode string  `json:"symbol_code"`
    Price      float64 `json:"price"`
    Change24h  float64 `json:"change_24h"`
    Volume24h  float64 `json:"volume_24h"`
    Timestamp  int64   `json:"timestamp"`
}
```

---

## 8. 与其他模块集成

### 8.1 箱体策略集成

```go
// 箱体突破时订阅监测
func (s *BoxStrategy) onBoxBreakout(box *model.PriceBox, direction string) {
    targetPrice := box.HighPrice
    condition := "cross_up"
    if direction == "down" {
        targetPrice = box.LowPrice
        condition = "cross_down"
    }

    subscriber := monitoring.Subscriber{
        Type: "box",
        ID:   box.ID,
        Callback: func(event monitoring.MonitorEvent) {
            s.handleBreakoutEvent(event, box)
        },
    }

    s.monitorFactory.Subscribe(box.SymbolID, targetPrice, condition, subscriber)
}

// 箱体关闭时取消订阅
func (s *BoxStrategy) onBoxClosed(box *model.PriceBox) {
    s.monitorFactory.Unsubscribe(box.SymbolID, "box", box.ID)
}
```

### 8.2 交易跟踪集成

```go
// 开仓时订阅止损止盈监测
func (e *TradeExecutor) subscribeMonitoring(track *model.TradeTrack) {
    // 订阅止损
    if track.StopLossPrice != nil {
        subscriber := monitoring.Subscriber{
            Type: "trade_track",
            ID:   track.ID,
            Callback: func(event monitoring.MonitorEvent) {
                e.CloseByStopLoss(track, event.CurrentPrice)
            },
        }

        if track.Direction == "long" {
            e.monitorFactory.Subscribe(track.SymbolID, *track.StopLossPrice, "less", subscriber)
        } else {
            e.monitorFactory.Subscribe(track.SymbolID, *track.StopLossPrice, "greater", subscriber)
        }
    }

    // 订阅止盈
    if track.TakeProfitPrice != nil {
        subscriber := monitoring.Subscriber{
            Type: "trade_track",
            ID:   track.ID,
            Callback: func(event monitoring.MonitorEvent) {
                e.CloseByTakeProfit(track, event.CurrentPrice)
            },
        }

        if track.Direction == "long" {
            e.monitorFactory.Subscribe(track.SymbolID, *track.TakeProfitPrice, "greater", subscriber)
        } else {
            e.monitorFactory.Subscribe(track.SymbolID, *track.TakeProfitPrice, "less", subscriber)
        }
    }
}
```

---

## 9. 文件结构

```
internal/service/monitoring/
├── monitor.go           # 监测器接口
├── factory.go          # 工厂模式
├── price_monitor.go    # 价格监测器
├── service.go          # 监测服务
├── event.go            # 事件定义
└── repository.go       # 监测数据访问
```

---

## 10. 配置项

```yaml
# config.yml
monitoring:
  price_check_interval: 1    # 价格检查间隔(秒)
  max_concurrent_monitors: 1000  # 最大并发监测数
  cleanup_interval: 300     # 清理间隔(秒)
```

---

## 11. 验收标准

### 11.1 功能验收

- [ ] 能创建价格监测器
- [ ] 能正确判断触发条件（greater, less, cross_up, cross_down）
- [ ] 订阅计数管理正确（多个策略共享）
- [ ] 退订后无订阅者时销毁监测器
- [ ] 服务重启后恢复活跃监测器

### 11.2 性能验收

- [ ] 支持1000+并发监测
- [ ] 价格检查延迟<1秒
- [ ] 事件处理无丢失

### 11.3 集成验收

- [ ] 箱体突破策略正确订阅/退订
- [ ] 交易跟踪正确订阅止盈止损
- [ ] WebSocket正确推送实时价格

---

## 12. 注意事项

1. **并发安全**：多协程访问监测器需要加锁
2. **事件丢失**：事件通道满时需要处理溢出
3. **资源清理**：监测器不再需要时应及时销毁
4. **服务重启**：需要正确恢复数据库中记录的活跃监测器

---

**前置依赖**: REQ-MARKET-001
**执行人**: 待分配
**预计工时**: 4小时
**实际完成时间**: 待填写
