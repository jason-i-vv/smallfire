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

// SignalBatcher 信号批处理器，在时间窗口内收集信号合并为一条汇总消息发送
type SignalBatcher struct {
	manager  *Manager
	window   time.Duration // 合并窗口
	maxBatch int           // 单批最大信号数
	ch       chan *models.Signal
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

// NewSignalBatcher 创建信号批处理器
func NewSignalBatcher(manager *Manager, window time.Duration, maxBatch int) *SignalBatcher {
	if window <= 0 {
		window = 3 * time.Second
	}
	if maxBatch <= 0 {
		maxBatch = 50
	}
	return &SignalBatcher{
		manager:  manager,
		window:   window,
		maxBatch: maxBatch,
		ch:       make(chan *models.Signal, maxBatch*2),
		stopCh:   make(chan struct{}),
	}
}

// Start 启动批处理器
func (b *SignalBatcher) Start() {
	b.wg.Add(1)
	go b.runLoop()
}

// Stop 停止批处理器
func (b *SignalBatcher) Stop() {
	close(b.stopCh)
	b.wg.Wait()
}

// Add 添加信号到批处理队列
func (b *SignalBatcher) Add(signal *models.Signal) {
	select {
	case b.ch <- signal:
	default:
		// 队列满，立即发送
		utils.Warn("批处理队列已满，立即发送信号",
			zap.String("symbol", signal.SymbolCode),
			zap.String("signal_type", signal.SignalType))
		b.manager.sendSignalImmediate(signal)
	}
}

func (b *SignalBatcher) runLoop() {
	defer b.wg.Done()

	var batch []*models.Signal
	timer := time.NewTimer(0)
	if !timer.Stop() {
		<-timer.C
	}

	for {
		select {
		case <-b.stopCh:
			// 退出前发送剩余信号
			if len(batch) > 0 {
				b.flush(batch)
			}
			timer.Stop()
			return

		case signal := <-b.ch:
			batch = append(batch, signal)

			// 达到最大批次立即发送
			if len(batch) >= b.maxBatch {
				timer.Stop()
				b.flush(batch)
				batch = nil
				continue
			}

			// 第一个信号到达时启动定时器
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

// flush 将一批信号合并发送
func (b *SignalBatcher) flush(signals []*models.Signal) {
	if len(signals) == 0 {
		return
	}

	// 单条信号直接发送
	if len(signals) == 1 {
		b.manager.sendSignalImmediate(signals[0])
		return
	}

	// 多条信号合并为汇总消息
	content := b.buildBatchContent(signals)

	for _, notifier := range b.manager.notifiers {
		if err := notifier.Send(content); err != nil {
			utils.Error("send batch signal notification failed", zap.Error(err))
		}
	}

	utils.Info("批量发送信号通知",
		zap.Int("signal_count", len(signals)))
}

// buildBatchContent 构建批量信号通知内容
func (b *SignalBatcher) buildBatchContent(signals []*models.Signal) *NotifyContent {
	var message string
	message += fmt.Sprintf("📊 **本轮策略分析产生 %d 个信号**\n\n", len(signals))

	for i, signal := range signals {
		direction := "做多"
		emoji := "🟢"
		if signal.Direction == "short" {
			direction = "做空"
			emoji = "🔴"
		}

		strength := ""
		for j := 0; j < signal.Strength; j++ {
			strength += "⭐"
		}

		signalType := getSignalTypeName(signal.SignalType)
		market := getMarketLabel(signal.SymbolCode)

		message += fmt.Sprintf("**%d.** %s %s %s %s\n", i+1, emoji, signal.SymbolCode, signalType, market)
		message += fmt.Sprintf("   周期: %s | 方向: %s | 强度: %s | 价格: %.4f\n\n",
			signal.Period, direction, strength, signal.Price)
	}

	return &NotifyContent{
		Title:   fmt.Sprintf("🔔 信号汇总 (%d)", len(signals)),
		Type:    models.NotifyTypeSignal,
		Message: message,
	}
}

// getSignalTypeName 获取信号类型中文名
func getSignalTypeName(signalType string) string {
	names := map[string]string{
		"box_breakout":        "箱体向上突破",
		"box_breakdown":       "箱体向下突破",
		"trend_reversal":      "趋势反转",
		"trend_retracement":   "趋势回撤",
		"resistance_break":    "阻力位突破",
		"support_break":       "支撑位跌破",
		"volume_surge":        "成交量放大",
		"price_surge_up":      "价格急涨",
		"price_surge_down":    "价格急跌",
		"upper_wick_reversal": "上引线反转",
		"lower_wick_reversal": "下引线反转",
		"fake_breakout_upper": "假突破上引线",
		"fake_breakout_lower": "假突破下引线",
		"momentum_bullish":    "连阳动量",
		"momentum_bearish":    "连阴动量",
		"morning_star":        "早晨之星",
		"evening_star":        "黄昏之星",
	}
	if name, ok := names[signalType]; ok {
		return name
	}
	return signalType
}

// getMarketLabel 从 SymbolCode 推断市场标签
func getMarketLabel(symbolCode string) string {
	code := strings.ToUpper(symbolCode)
	if strings.HasSuffix(code, "USDT") {
		return "[加密]"
	}
	// A股代码以数字开头，通常6位
	if len(code) == 6 && code[0] >= '0' && code[0] <= '9' {
		return "[A股]"
	}
	// 其余识别为美股
	return "[美股]"
}
