package notification

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/internal/repository"
	"github.com/smallfire/starfire/internal/service/trading"
	"github.com/smallfire/starfire/pkg/utils"
)

// Manager 通知管理器
type Manager struct {
	notifiers       map[string]Notifier
	summarySvc      *SummaryService
	notifyRepo      repository.NotificationRepo
	batcher         *SignalBatcher
	oppBatcher      *OpportunityBatcher
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

	m.batcher = NewSignalBatcher(m, 3*time.Second, 50)
	m.batcher.Start()

	m.oppBatcher = NewOpportunityBatcher(m, 5*time.Second, 20)
	m.oppBatcher.Start()

	return m
}

// Stop 停止管理器（含批处理器）
func (m *Manager) Stop() {
	m.oppBatcher.Stop()
	m.batcher.Stop()
}

// SendToAll 发送到所有渠道
func (m *Manager) SendToAll(content *NotifyContent) {
	for _, notifier := range m.notifiers {
		go func(n Notifier) {
			if err := n.Send(content); err != nil {
				utils.Error("send notification failed", zap.Error(err))
			}
		}(notifier)
	}
}

// SendSignal 发送信号通知（走批处理，合并为汇总消息）
func (m *Manager) SendSignal(signal *models.Signal) error {
	m.batcher.Add(signal)
	return nil
}

// SendOpportunity 发送交易机会通知（走批处理，合并为汇总消息）
func (m *Manager) SendOpportunity(opp *models.TradingOpportunity) error {
	m.oppBatcher.Add(opp)
	return nil
}

// sendOpportunityImmediate 立即发送单条交易机会通知（批处理器内部调用）
func (m *Manager) sendOpportunityImmediate(opp *models.TradingOpportunity) {
	for _, notifier := range m.notifiers {
		if err := notifier.SendOpportunityNotification(opp); err != nil {
			utils.Error("send opportunity notification failed", zap.Error(err))
		}
	}
}

// sendSignalImmediate 立即发送单条信号通知（批处理器内部调用）
func (m *Manager) sendSignalImmediate(signal *models.Signal) {
	for _, notifier := range m.notifiers {
		if err := notifier.SendSignalNotification(signal); err != nil {
			utils.Error("send signal notification failed", zap.Error(err))
		}
	}
}

// SendTradeOpened 发送开仓通知
func (m *Manager) SendTradeOpened(track *models.TradeTrack) {
	for _, notifier := range m.notifiers {
		go func(n Notifier) {
			if err := n.SendTradeOpenedNotification(track); err != nil {
				utils.Error("send trade opened notification failed", zap.Error(err))
			}
		}(notifier)
	}
}

// SendTradeClosed 发送平仓通知
func (m *Manager) SendTradeClosed(track *models.TradeTrack) {
	for _, notifier := range m.notifiers {
		go func(n Notifier) {
			if err := n.SendTradeClosedNotification(track); err != nil {
				utils.Error("send trade closed notification failed", zap.Error(err))
			}
		}(notifier)
	}
}

// SummaryService 汇总服务
type SummaryService struct {
	notifier    *FeishuNotifier
	statsService *trading.StatisticsService
	config      *FeishuConfig
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

func NewSummaryService(notifier *FeishuNotifier, statsService *trading.StatisticsService, config *FeishuConfig) *SummaryService {
	return &SummaryService{
		notifier:    notifier,
		statsService: statsService,
		config:      config,
	}
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

func (s *SummaryService) waitUntilNextSummary(times []string) {
	now := time.Now().In(cstZone)
	nextTime := s.findNextSummaryTime(times, now)
	if nextTime.IsZero() {
		// 没有找到今天的时间，明天再检查
		tomorrow := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, cstZone)
		time.Sleep(tomorrow.Sub(now))
		return
	}

	duration := nextTime.Sub(now)
	if duration <= 0 {
		return
	}

	select {
	case <-s.stopCh:
		return
	case <-time.After(duration):
	}
}

func (s *SummaryService) findNextSummaryTime(times []string, now time.Time) time.Time {
	year, month, day := now.Date()
	location := now.Location()

	var nextTime time.Time

	for _, t := range times {
		hour, minute, err := parseTime(t)
		if err != nil {
			continue
		}

		target := time.Date(year, month, day, hour, minute, 0, 0, location)

		if target.After(now) && (nextTime.IsZero() || target.Before(nextTime)) {
			nextTime = target
		}
	}

	return nextTime
}

func (s *SummaryService) parseSummaryTimes(timeStrings []string) []string {
	var validTimes []string

	for _, t := range timeStrings {
		if _, _, err := parseTime(t); err == nil {
			validTimes = append(validTimes, t)
		}
	}

	return validTimes
}

func parseTime(timeStr string) (int, int, error) {
	var h, m int
	_, err := fmt.Sscanf(timeStr, "%d:%d", &h, &m)
	if err != nil {
		return 0, 0, err
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, 0, fmt.Errorf("invalid time format: %s", timeStr)
	}
	return h, m, nil
}

func (s *SummaryService) sendSummary() {
	stats, err := s.getTodayStats()
	if err != nil {
		utils.Error("get today stats failed", zap.Error(err))
		return
	}

	if err := s.notifier.SendSummaryNotification(stats); err != nil {
		utils.Error("send summary failed", zap.Error(err))
	}
}

func (s *SummaryService) getTodayStats() (*SummaryStats, error) {
	// 获取今日统计数据
	now := time.Now().In(cstZone)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, cstZone)

	stats, err := s.statsService.GetStatistics(&today, nil)
	if err != nil {
		return nil, err
	}

	summaryStats := &SummaryStats{
		TodayTrades:    stats.TotalTrades,
		WinTrades:      stats.WinTrades,
		LossTrades:     stats.LossTrades,
		WinRate:        stats.WinRate,
		TotalPnL:       stats.TotalPnL,
		MaxDrawdownPct: stats.MaxDrawdownPct,
		InitialCapital: stats.InitialCapital,
		CurrentCapital: stats.CurrentCapital,
		TotalReturn:    stats.TotalReturn,
		TodaySignals:   0, // TODO: 从信号服务获取今日信号数
		OpenPositions:  0, // TODO: 从持仓服务获取活跃持仓数
	}

	return summaryStats, nil
}
