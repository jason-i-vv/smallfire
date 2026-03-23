// internal/service/monitoring/factory.go
package monitoring

import (
	"fmt"
	"sync"

	"github.com/smallfire/starfire/internal/repository"
)

type Factory struct {
	monitors    map[int64]Monitor  // monitor_id -> Monitor
	tickerRepo  repository.TickerRepo
	eventChan   chan MonitorEvent
	mu          sync.RWMutex
	maxMonitors int
}

func NewFactory(tickerRepo repository.TickerRepo, maxMonitors int) *Factory {
	f := &Factory{
		monitors:    make(map[int64]Monitor),
		tickerRepo:  tickerRepo,
		eventChan:   make(chan MonitorEvent, 1000),
		maxMonitors: maxMonitors,
	}
	return f
}

// Subscribe 订阅价格监测
func (f *Factory) Subscribe(symbolID int64, targetPrice float64, condition string, subscriber Subscriber) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// 检查是否已存在相同条件的监测器
	var monitor Monitor
	for _, m := range f.monitors {
		if m.SymbolID() == symbolID && m.Type() == MonitorTypePrice {
			pm, ok := m.(*PriceMonitor)
			if ok && pm.condition == condition && pm.targetPrice == targetPrice {
				monitor = m
				break
			}
		}
	}

	if monitor == nil {
		// 检查最大数量
		if len(f.monitors) >= f.maxMonitors {
			return fmt.Errorf("已达到最大监测数量限制")
		}

		// 创建新监测器
		monitor = NewPriceMonitor(symbolID, targetPrice, condition, f.eventChan)
		f.monitors[monitor.ID()] = monitor
	}

	// 添加订阅者
	monitor.AddSubscriber(subscriber)

	return nil
}

// Unsubscribe 取消订阅
func (f *Factory) Unsubscribe(symbolID int64, subscriberType string, subscriberID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// 查找该标的的所有监测器
	var toRemove []int64
	for id, monitor := range f.monitors {
		if monitor.SymbolID() == symbolID {
			monitor.RemoveSubscriber(subscriberType, subscriberID)
			if monitor.SubscriberCount() == 0 {
				toRemove = append(toRemove, id)
			}
		}
	}

	// 移除无订阅者的监测器
	for _, id := range toRemove {
		delete(f.monitors, id)
	}

	return nil
}

// GetActiveMonitors 获取所有活跃监测器
func (f *Factory) GetActiveMonitors() []Monitor {
	f.mu.RLock()
	defer f.mu.RUnlock()

	var monitors []Monitor
	for _, monitor := range f.monitors {
		if monitor.IsActive() {
			monitors = append(monitors, monitor)
		}
	}

	return monitors
}

// EventChan 获取事件通道
func (f *Factory) EventChan() chan MonitorEvent {
	return f.eventChan
}
