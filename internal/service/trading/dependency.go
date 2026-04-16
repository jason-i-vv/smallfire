package trading

import (
	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/internal/repository"
	"go.uber.org/zap"
)

// Dependency 交易服务依赖
type Dependency struct {
	TrackRepo  repository.TradeTrackRepo
	SignalRepo repository.SignalRepo
	OppRepo    repository.OpportunityRepo
	StatsRepo  repository.SignalTypeStatsRepo
	Logger     *zap.Logger
}

// MonitorFactory 价格监控工厂接口
type MonitorFactory interface {
	Subscribe(track *models.TradeTrack)
	Unsubscribe(track *models.TradeTrack)
}

// Notifier 通知器接口
type Notifier interface {
	SendTradeOpened(track *models.TradeTrack)
	SendTradeClosed(track *models.TradeTrack)
}
