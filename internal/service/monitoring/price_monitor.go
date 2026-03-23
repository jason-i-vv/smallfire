// internal/service/monitoring/price_monitor.go
package monitoring

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/smallfire/starfire/pkg/utils"
)

var monitorIDCounter int64

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
	eventChan     chan MonitorEvent
	mu            sync.RWMutex
}

func NewPriceMonitor(symbolID int64, targetPrice float64, condition string, eventChan chan MonitorEvent) *PriceMonitor {
	return &PriceMonitor{
		id:          atomic.AddInt64(&monitorIDCounter, 1),
		symbolID:    symbolID,
		targetPrice: targetPrice,
		condition:   condition,
		isActive:    true,
		createdAt:   time.Now(),
		eventChan:   eventChan,
	}
}

func (m *PriceMonitor) ID() int64 {
	return m.id
}

func (m *PriceMonitor) SymbolID() int64 {
	return m.symbolID
}

func (m *PriceMonitor) Type() string {
	return MonitorTypePrice
}

func (m *PriceMonitor) GetTargetPrice() float64 {
	return m.targetPrice
}

func (m *PriceMonitor) GetSubscribers() []Subscriber {
	m.mu.RLock()
	defer m.mu.RUnlock()

	subs := make([]Subscriber, len(m.subscribers))
	copy(subs, m.subscribers)
	return subs
}

func (m *PriceMonitor) IsActive() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isActive
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
		utils.Warn("monitor event channel is full")
	}

	// 通知所有订阅者
	for _, sub := range m.subscribers {
		go sub.Callback(event)
	}
}
