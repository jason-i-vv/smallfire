package notification

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/pkg/utils"
)

// OpportunityBatcher 交易机会批处理器，在时间窗口内收集机会合并为一条消息发送
type OpportunityBatcher struct {
	manager  *Manager
	window   time.Duration // 合并窗口
	maxBatch int           // 单批最大机会数
	ch       chan *models.TradingOpportunity
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

// NewOpportunityBatcher 创建交易机会批处理器
func NewOpportunityBatcher(manager *Manager, window time.Duration, maxBatch int) *OpportunityBatcher {
	if window <= 0 {
		window = 5 * time.Second
	}
	if maxBatch <= 0 {
		maxBatch = 20
	}
	return &OpportunityBatcher{
		manager:  manager,
		window:   window,
		maxBatch: maxBatch,
		ch:       make(chan *models.TradingOpportunity, maxBatch*2),
		stopCh:   make(chan struct{}),
	}
}

// Start 启动批处理器
func (b *OpportunityBatcher) Start() {
	b.wg.Add(1)
	go b.runLoop()
}

// Stop 停止批处理器
func (b *OpportunityBatcher) Stop() {
	close(b.stopCh)
	b.wg.Wait()
}

// Add 添加交易机会到批处理队列
func (b *OpportunityBatcher) Add(opp *models.TradingOpportunity) {
	select {
	case b.ch <- opp:
	default:
		utils.Warn("交易机会批处理队列已满，立即发送",
			zap.String("symbol", opp.SymbolCode),
			zap.Int("score", opp.Score))
		b.manager.sendOpportunityImmediate(opp)
	}
}

func (b *OpportunityBatcher) runLoop() {
	defer b.wg.Done()

	var batch []*models.TradingOpportunity
	timer := time.NewTimer(0)
	if !timer.Stop() {
		<-timer.C
	}

	for {
		select {
		case <-b.stopCh:
			if len(batch) > 0 {
				b.flush(batch)
			}
			timer.Stop()
			return

		case opp := <-b.ch:
			batch = append(batch, opp)

			if len(batch) >= b.maxBatch {
				timer.Stop()
				b.flush(batch)
				batch = nil
				continue
			}

			if len(batch) == 1 {
				timer.Reset(b.window)
			}

		case <-timer.C:
			if len(batch) > 0 {
				b.flush(batch)
				batch = nil
			}
		}
	}
}

// flush 将一批交易机会合并发送
func (b *OpportunityBatcher) flush(opps []*models.TradingOpportunity) {
	if len(opps) == 0 {
		return
	}

	// 单条也走合并格式，避免消息碎片化导致飞书频率限制
	if len(opps) == 1 {
		content := b.buildBatchContent(opps)
		for _, notifier := range b.manager.notifiers {
			if err := notifier.Send(content); err != nil {
				utils.Error("send opportunity notification failed", zap.Error(err))
			}
		}
		return
	}

	// 多条合并为汇总消息
	content := b.buildBatchContent(opps)

	for _, notifier := range b.manager.notifiers {
		if err := notifier.Send(content); err != nil {
			utils.Error("send batch opportunity notification failed", zap.Error(err))
		}
	}

	utils.Info("批量发送交易机会通知",
		zap.Int("opportunity_count", len(opps)))
}

// buildBatchContent 构建批量交易机会通知内容
func (b *OpportunityBatcher) buildBatchContent(opps []*models.TradingOpportunity) *NotifyContent {
	message := fmt.Sprintf("🎯 **本轮分析发现 %d 个交易机会**\n\n", len(opps))

	for i, opp := range opps {
		direction := "做多"
		emoji := "🟢"
		if opp.Direction == "short" {
			direction = "做空"
			emoji = "🔴"
		}

		market := getMarketLabel(opp.SymbolCode)

		// 共识信号类型（去重）
		strategyNames := make(map[string]bool)
		for _, cd := range opp.ConfluenceDirections {
			parts := strings.SplitN(cd, ":", 2)
			if len(parts) >= 1 {
				strategyNames[getSignalTypeName(parts[0])] = true
			}
		}
		strategies := make([]string, 0, len(strategyNames))
		for name := range strategyNames {
			strategies = append(strategies, name)
		}

		message += fmt.Sprintf("**%d.** %s %s %s %s\n", i+1, emoji, market, opp.SymbolCode, direction)
		message += fmt.Sprintf("   评分: %d/100 | 信号数: %d | 周期: %s\n", opp.Score, opp.SignalCount, opp.Period)

		if opp.SuggestedEntry != nil {
			message += fmt.Sprintf("   入场: %.4f", *opp.SuggestedEntry)
		}
		if opp.SuggestedStopLoss != nil {
			message += fmt.Sprintf(" | 止损: %.4f", *opp.SuggestedStopLoss)
		}
		if opp.SuggestedTakeProfit != nil {
			message += fmt.Sprintf(" | 止盈: %.4f", *opp.SuggestedTakeProfit)
		}
		if opp.SuggestedEntry != nil || opp.SuggestedStopLoss != nil || opp.SuggestedTakeProfit != nil {
			message += "\n"
		}

		if len(strategies) > 0 {
			message += fmt.Sprintf("   策略: %s\n", strings.Join(strategies, " + "))
		}
		message += "\n"
	}

	return &NotifyContent{
		Title:   fmt.Sprintf("🎯 交易机会汇总 (%d)", len(opps)),
		Type:    "opportunity",
		Message: message,
	}
}
