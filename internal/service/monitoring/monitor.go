// internal/service/monitoring/monitor.go
package monitoring

import "time"

// Monitor 监测器接口
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

	// 添加订阅者
	AddSubscriber(sub Subscriber)

	// 移除订阅者
	RemoveSubscriber(subType string, subID int64)

	// 获取订阅者数量
	SubscriberCount() int
}

// Subscriber 订阅者信息
type Subscriber struct {
	Type     string    // box, trade_track
	ID       int64     // 关联ID
	Callback func(MonitorEvent) // 回调函数
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

// MonitorType 常量
const (
	MonitorTypePrice = "price"
	MonitorTypeBox   = "box"
	MonitorTypeTrend = "trend"
)

// ConditionType 常量
const (
	ConditionGreater  = "greater"   // 价格大于目标
	ConditionLess     = "less"      // 价格小于目标
	ConditionCrossUp  = "cross_up"  // 价格从下往上穿过
	ConditionCrossDown = "cross_down" // 价格从上往下穿过
)
