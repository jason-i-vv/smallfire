// internal/service/monitoring/service.go
package monitoring

import (
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/internal/repository"
	"github.com/smallfire/starfire/pkg/utils"
)

type Service struct {
	factory     *Factory
	tickerRepo  repository.TickerRepo
	monitorRepo repository.MonitorRepo
	interval    time.Duration
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

func NewService(factory *Factory, tickerRepo repository.TickerRepo, monitorRepo repository.MonitorRepo, interval time.Duration) *Service {
	return &Service{
		factory:     factory,
		tickerRepo:  tickerRepo,
		monitorRepo: monitorRepo,
		interval:    interval,
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

	utils.Info("monitoring service started")
}

func (s *Service) Stop() {
	close(s.stopCh)
	s.wg.Wait()
	utils.Info("monitoring service stopped")
}

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
			utils.Info("monitor triggered",
				zap.Int64("monitor_id", monitor.ID()),
				zap.Int64("symbol_id", symbolID),
				zap.Float64("target", monitor.GetTargetPrice()),
				zap.Float64("current", currentPrice),
			)

			// 更新数据库状态
			s.updateMonitorTriggered(monitor.ID(), currentPrice)
		}
	}
}

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

func (s *Service) handleEvent(event MonitorEvent) {
	// 根据事件类型分发处理
	switch event.EventType {
	case ConditionCrossUp, ConditionCrossDown:
		// 处理突破事件
		s.handleBreakoutEvent(event)
	case ConditionGreater, ConditionLess:
		// 处理价格到达事件
		s.handlePriceReachedEvent(event)
	}
}

func (s *Service) handleBreakoutEvent(event MonitorEvent) {
	// 获取订阅该监测器的所有订阅者类型
	// 根据类型调用不同的处理逻辑
	utils.Info("breakout event",
		zap.String("symbol_code", event.SymbolCode),
		zap.Float64("price", event.CurrentPrice),
	)
}

func (s *Service) handlePriceReachedEvent(event MonitorEvent) {
	utils.Info("price reached event",
		zap.String("symbol_code", event.SymbolCode),
		zap.Float64("price", event.CurrentPrice),
	)
}

func (s *Service) recoverActiveMonitors() {
	// 从数据库恢复活跃监测器
	monitors, err := s.monitorRepo.GetActiveMonitors()
	if err != nil {
		utils.Error("recover monitors failed", zap.Error(err))
		return
	}

	for _, m := range monitors {
		// 注意：这里只恢复监测器记录，不恢复订阅者（订阅者需要由各个模块重新订阅）
		utils.Info("recovered monitor",
			zap.Int64("id", m.ID),
			zap.String("symbol", m.SymbolCode),
		)
	}

	utils.Info("recovered active monitors", zap.Int("count", len(monitors)))
}

func (s *Service) updateMonitorTriggered(monitorID int64, currentPrice float64) {
	now := time.Now()
	err := s.monitorRepo.UpdateTriggered(monitorID, currentPrice, &now)
	if err != nil {
		utils.Error("update monitor triggered failed", zap.Error(err))
	}
}

// Subscribe 订阅价格监测（供外部调用）
func (s *Service) Subscribe(symbolID int64, targetPrice float64, condition string, subscriber Subscriber) error {
	// 先保存到数据库
	monitor := &models.Monitoring{
		SymbolID:        symbolID,
		MonitorType:     MonitorTypePrice,
		TargetPrice:     &targetPrice,
		ConditionType:   condition,
		SubscriberCount: 1,
		IsActive:        true,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	err := s.monitorRepo.Create(monitor)
	if err != nil {
		return err
	}

	// 再添加到工厂
	return s.factory.Subscribe(symbolID, targetPrice, condition, subscriber)
}

// Unsubscribe 取消订阅（供外部调用）
func (s *Service) Unsubscribe(symbolID int64, subscriberType string, subscriberID int64) error {
	return s.factory.Unsubscribe(symbolID, subscriberType, subscriberID)
}
