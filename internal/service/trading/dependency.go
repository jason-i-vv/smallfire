package trading

import (
	"github.com/smallfire/starfire/internal/repository"
	"go.uber.org/zap"
)

// Dependency 交易服务依赖
type Dependency struct {
	TrackRepo  repository.TradeTrackRepo
	SignalRepo repository.SignalRepo
	Logger     *zap.Logger
}

// MonitorFactory 价格监控工厂接口
type MonitorFactory interface {
	Subscribe(track interface{})
	Unsubscribe(track interface{})
}

// Notifier 通知器接口
type Notifier interface {
	SendTradeOpened(track interface{})
	SendTradeClosed(track interface{})
}
