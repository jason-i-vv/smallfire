package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/internal/repository"
	"github.com/smallfire/starfire/internal/service/notification"
	"go.uber.org/zap"
)

const (
	minWatchContext  = 40
	defaultWatchLimit = 120
	maxWatchLimit    = 200
)

// WatchEngine 统一分析引擎
// 使用 Claude SDK + 会话机制实现增量分析
type WatchEngine struct {
	claude    *ClaudeClient
	registry  *SkillRegistry
	klineRepo repository.KlineRepo
	notifier  *notification.Manager
	logger    *zap.Logger
}

// NewWatchEngine 创建统一分析引擎
func NewWatchEngine(
	claude *ClaudeClient,
	registry *SkillRegistry,
	klineRepo repository.KlineRepo,
	notifier *notification.Manager,
	logger *zap.Logger,
) *WatchEngine {
	return &WatchEngine{
		claude:    claude,
		registry:  registry,
		klineRepo: klineRepo,
		notifier:  notifier,
		logger:    logger,
	}
}

// AnalyzeTarget 分析单个观察仓（增量会话模式）
func (e *WatchEngine) AnalyzeTarget(ctx context.Context, target *models.AIWatchTarget) error {
	if e.claude == nil {
		return fmt.Errorf("Claude 客户端未初始化")
	}

	skill, ok := e.registry.Get(target.SkillName)
	if !ok {
		return fmt.Errorf("未找到策略: %s", target.SkillName)
	}

	symbolID := 0
	if target.SymbolID != nil {
		symbolID = *target.SymbolID
	}
	if symbolID <= 0 {
		return fmt.Errorf("symbol_id 无效")
	}

	// 1. 获取最新 K 线
	limit := normalizeLimit(target.Limit, defaultWatchLimit, maxWatchLimit)
	klines, err := e.klineRepo.GetLatestN(symbolID, target.Period, limit)
	if err != nil {
		return fmt.Errorf("获取 K 线失败: %w", err)
	}
	if len(klines) < minWatchContext+1 {
		return fmt.Errorf("K 线数量不足，至少需要 %d 根", minWatchContext+1)
	}

	sort.Slice(klines, func(i, j int) bool {
		return klines[i].OpenTime.Before(klines[j].OpenTime)
	})

	// 2. 加载已有会话
	conv, err := e.claude.LoadConversation(target.ID)
	if err != nil {
		e.logger.Warn("加载会话失败，将重新创建", zap.Int("target_id", target.ID), zap.Error(err))
		conv = nil
	}

	var raw string

	if conv == nil {
		// 3a. 首次分析：全量 K 线 + 完整 system prompt
		observationStart := len(klines) - 12
		if observationStart < minWatchContext {
			observationStart = minWatchContext
		}

		conv = &ClaudeConversation{
			SystemPrompt: skill.SystemPrompt(target.MarketCode),
			TargetID:     target.ID,
			SkillName:    target.SkillName,
			CreatedAt:    time.Now().UnixMilli(),
			Messages: []ClaudeMessage{
				{
					Role:    "user",
					Content: e.buildHeader(target) + skill.BuildFirstMessage(klines, observationStart),
				},
			},
		}

		e.logger.Info("首次分析，发送全量 K 线",
			zap.String("symbol", target.SymbolCode),
			zap.String("skill", target.SkillName),
			zap.Int("kline_count", len(klines)))
	} else {
		// 3b. 增量分析：只发新 K 线
		newKlines := e.findNewKlines(klines, conv)
		if len(newKlines) == 0 {
			return fmt.Errorf("没有新 K 线数据")
		}

		conv.Messages = append(conv.Messages, ClaudeMessage{
			Role:    "user",
			Content: skill.BuildIncrementalMessage(newKlines),
		})

		e.logger.Info("增量分析，发送新 K 线",
			zap.String("symbol", target.SymbolCode),
			zap.Int("new_klines", len(newKlines)),
			zap.Int("total_messages", len(conv.Messages)))
	}

	// 4. 调用 Claude API
	raw, err = e.claude.Chat(ctx, conv.SystemPrompt, conv.Messages)
	if err != nil {
		return fmt.Errorf("Claude 分析失败: %w", err)
	}

	// 5. 解析结果
	steps, err := skill.ParseResponse(raw)
	if err != nil {
		// 解析失败，仍然保存 AI 原始回复到会话
		e.logger.Warn("解析 AI 响应失败", zap.Error(err))
		conv.Messages = append(conv.Messages, ClaudeMessage{Role: "assistant", Content: raw})
		_ = e.claude.SaveConversation(conv)
		return fmt.Errorf("解析 AI 响应失败: %w", err)
	}

	// 6. 填充 K 线数据到 steps
	steps = e.fillKlineData(steps, klines)

	// 7. 保存 AI 回复到会话
	conv.Messages = append(conv.Messages, ClaudeMessage{Role: "assistant", Content: raw})

	// 8. 会话压缩
	if len(conv.Messages) > 80 { // 40 轮 * 2 条
		e.logger.Info("压缩会话历史",
			zap.Int("target_id", target.ID),
			zap.Int("messages", len(conv.Messages)))
		conv = e.claude.CompressConversation(conv, 20)
	}

	if err := e.claude.SaveConversation(conv); err != nil {
		e.logger.Error("保存会话失败", zap.Error(err))
	}

	// 9. 合并结果到 target
	newStepsJSON, _ := json.Marshal(steps)
	target.Result = mergeStepsJSON(target.Result, newStepsJSON)
	target.DataStatus = "ready"
	target.ErrorMessage = ""
	now := time.Now().UnixMilli()
	target.LastRunAt = &now

	// 10. 判断是否关闭（连续 3 根 invalid 才关闭）
	if shouldDisableTracking(target.Result) {
		target.Enabled = false
		e.logger.Info("趋势连续失效，自动关闭 AI 跟踪",
			zap.String("symbol", target.SymbolCode),
			zap.String("skill", target.SkillName))
	}

	// 11. 检查是否有可操作买点，发送通知
	if target.SendFeishu && e.notifier != nil {
		for i := range steps {
			step := &steps[i]
			if step.Decision == "alert" && step.BuyPoint == "ready" && step.Confidence >= 70 {
				e.notifier.SendToAll(e.buildNotification(target, step))
				break
			}
		}
	}

	return nil
}

// ResetTarget 重置观察仓的会话（下次分析将从零开始）
func (e *WatchEngine) ResetTarget(targetID int) error {
	return e.claude.ResetConversation(targetID)
}

// findNewKlines 找出尚未发送过的 K 线
func (e *WatchEngine) findNewKlines(klines []models.Kline, conv *ClaudeConversation) []models.Kline {
	if len(conv.Messages) == 0 {
		return klines
	}

	// 从最后一条 user 消息中提取最后一条 K 线的时间
	lastUserMsg := ""
	for i := len(conv.Messages) - 1; i >= 0; i-- {
		if conv.Messages[i].Role == "user" {
			lastUserMsg = conv.Messages[i].Content
			break
		}
	}

	if lastUserMsg == "" {
		return klines[len(klines)-1:] // 默认只取最后 1 根
	}

	// 找到 K 线中最晚的时间，之后的都是新的
	lastKlineTime := time.Time{}
	for _, k := range klines {
		if k.OpenTime.After(lastKlineTime) {
			lastKlineTime = k.OpenTime
		}
	}

	// 简单策略：取最后几根 K 线（最近 1-3 根未分析的）
	// 通过比较会话中的消息数量和 K 线数量来估算
	estimatedAnalyzed := len(conv.Messages) / 2 // 每轮大约 2 条消息
	if estimatedAnalyzed > len(klines) {
		estimatedAnalyzed = len(klines) - 1
	}

	// 保守取最后 3 根作为"新 K 线"，确保不遗漏
	newCount := 3
	if newCount > len(klines) {
		newCount = len(klines)
	}

	return klines[len(klines)-newCount:]
}

// fillKlineData 填充 K 线时间和价格到 steps
func (e *WatchEngine) fillKlineData(steps []AnalysisStep, klines []models.Kline) []AnalysisStep {
	// 为增量分析的 steps 补充 kline_time 和 close_price
	for i := range steps {
		if steps[i].KlineTime == 0 && len(klines) > 0 {
			// 从最后几根 K 线中匹配
			klineIdx := len(klines) - len(steps) + i
			if klineIdx >= 0 && klineIdx < len(klines) {
				steps[i].KlineTime = klines[klineIdx].OpenTime.UnixMilli()
				steps[i].ClosePrice = klines[klineIdx].ClosePrice
			}
		}
	}
	return steps
}

// buildHeader 构建消息头（标的元信息）
func (e *WatchEngine) buildHeader(target *models.AIWatchTarget) string {
	return fmt.Sprintf("标的: %s\n市场: %s\n周期: %s\n方向: 做多\n\n", target.SymbolCode, target.MarketCode, target.Period)
}

// shouldDisableTracking 检查最近的 steps 是否连续 3 根 invalid
func shouldDisableTracking(resultJSON json.RawMessage) bool {
	if len(resultJSON) == 0 {
		return false
	}

	var result struct {
		Steps []json.RawMessage `json:"steps"`
	}
	if json.Unmarshal(resultJSON, &result) != nil || len(result.Steps) == 0 {
		return false
	}

	// 检查最后 3 根
	latest := result.Steps
	if len(latest) > 3 {
		latest = latest[len(latest)-3:]
	}

	for _, raw := range latest {
		var step struct {
			Decision string `json:"decision"`
		}
		if json.Unmarshal(raw, &step) != nil {
			return false
		}
		if step.Decision != "invalid" {
			return false
		}
	}
	return len(latest) >= 3
}

func (e *WatchEngine) buildNotification(target *models.AIWatchTarget, step *AnalysisStep) *notification.NotifyContent {
	message := fmt.Sprintf("标的: %s\n周期: %s\n策略: %s\n置信度: %d\n理由: %s",
		target.SymbolCode, target.Period, target.SkillName, step.Confidence, step.Reasoning)
	if step.EntryPrice != nil {
		message += fmt.Sprintf("\n建议入场: %.6g", *step.EntryPrice)
	}
	if step.StopLoss != nil {
		message += fmt.Sprintf("\n建议止损: %.6g", *step.StopLoss)
	}
	if step.TakeProfit != nil {
		message += fmt.Sprintf("\n建议止盈: %.6g", *step.TakeProfit)
	}
	if len(step.RiskNotes) > 0 {
		message += "\n风险提示: " + formatRiskNotes(step.RiskNotes)
	}

	return &notification.NotifyContent{
		Title:   fmt.Sprintf("AI 买点提醒 %s (%s)", target.SymbolCode, target.SkillName),
		Type:    "opportunity",
		Message: message,
		Data: map[string]interface{}{
			"skill":      target.SkillName,
			"period":     target.Period,
			"confidence": step.Confidence,
			"kline_time": time.UnixMilli(step.KlineTime).In(time.FixedZone("UTC+8", 8*60*60)).Format("2006-01-02 15:04:05"),
		},
	}
}

func formatRiskNotes(notes []string) string {
	result := ""
	for i, n := range notes {
		if i > 0 {
			result += "；"
		}
		result += n
	}
	return result
}
