package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/internal/repository"
	"github.com/smallfire/starfire/internal/service/notification"
	"go.uber.org/zap"
)

const (
	defaultTrendPullbackLimit     = 120
	defaultTrendPullbackStepLimit = 12
	maxTrendPullbackLimit         = 200
	maxTrendPullbackStepLimit     = 30
	minTrendPullbackContext       = 40
	trendPullbackMaxOutputTokens  = 6000
)

// TrendPullbackAnalyzer 手动趋势回调 AI 分析器。
// 第一版用于快速验证：用户手动选择强趋势标的和周期，系统批量回放 K 线寻找回调买点。
type TrendPullbackAnalyzer struct {
	client    *AIClient
	klineRepo repository.KlineRepo
	notifier  *notification.Manager
	logger    *zap.Logger
}

type TrendPullbackRequest struct {
	SymbolID   int    `json:"symbol_id"`
	SymbolCode string `json:"symbol_code"`
	MarketCode string `json:"market_code"`
	Period     string `json:"period"`
	Direction  string `json:"direction"`
	Limit      int    `json:"limit"`
	StepLimit  int    `json:"step_limit"`
	SendFeishu bool   `json:"send_feishu"`
}

type TrendPullbackResponse struct {
	SymbolCode string                    `json:"symbol_code"`
	MarketCode string                    `json:"market_code"`
	Period     string                    `json:"period"`
	Direction  string                    `json:"direction"`
	Analyzed   int                       `json:"analyzed"`
	Found      bool                      `json:"found"`
	Best       *TrendPullbackStepResult  `json:"best,omitempty"`
	Steps      []TrendPullbackStepResult `json:"steps"`
	Notified   bool                      `json:"notified"`
}

type TrendPullbackStepResult struct {
	KlineTime         int64    `json:"kline_time"`
	ClosePrice        float64  `json:"close_price"`
	TrendState        string   `json:"trend_state"`
	PullbackState     string   `json:"pullback_state"`
	BuyPoint          string   `json:"buy_point"`
	Decision          string   `json:"decision"`
	EntryPrice        *float64 `json:"entry_price,omitempty"`
	StopLoss          *float64 `json:"stop_loss,omitempty"`
	TakeProfit        *float64 `json:"take_profit,omitempty"`
	InvalidationLevel *float64 `json:"invalidation_level,omitempty"`
	Confidence        int      `json:"confidence"`
	Missed            bool     `json:"missed"`
	MissedKlineIndex  *int     `json:"missed_kline_index,omitempty"`
	Reasoning         string   `json:"reasoning"`
	RiskNotes         []string `json:"risk_notes"`
	Raw               string   `json:"raw,omitempty"`
}

type trendPullbackAIResult struct {
	KlineIndex        int      `json:"kline_index"`
	TrendState        string   `json:"trend_state"`
	PullbackState     string   `json:"pullback_state"`
	BuyPoint          string   `json:"buy_point"`
	Decision          string   `json:"decision"`
	EntryPrice        *float64 `json:"entry_price"`
	StopLoss          *float64 `json:"stop_loss"`
	TakeProfit        *float64 `json:"take_profit"`
	InvalidationLevel *float64 `json:"invalidation_level"`
	Confidence        int      `json:"confidence"`
	Missed            bool     `json:"missed"`
	MissedKlineIndex  *int     `json:"missed_kline_index"`
	Reasoning         string   `json:"reasoning"`
	RiskNotes         []string `json:"risk_notes"`
}

type trendPullbackBatchAIResult struct {
	Steps []trendPullbackAIResult `json:"steps"`
}

func NewTrendPullbackAnalyzer(client *AIClient, klineRepo repository.KlineRepo, notifier *notification.Manager, logger *zap.Logger) *TrendPullbackAnalyzer {
	return &TrendPullbackAnalyzer{
		client:    client,
		klineRepo: klineRepo,
		notifier:  notifier,
		logger:    logger,
	}
}

func (a *TrendPullbackAnalyzer) Analyze(ctx context.Context, req TrendPullbackRequest) (*TrendPullbackResponse, error) {
	if a == nil || a.client == nil {
		return nil, fmt.Errorf("AI 分析服务未启用")
	}
	if req.SymbolID <= 0 {
		return nil, fmt.Errorf("symbol_id 不能为空")
	}
	if req.Period == "" {
		req.Period = "1h"
	}
	if req.Direction == "" {
		req.Direction = models.DirectionLong
	}
	if req.Direction != models.DirectionLong {
		return nil, fmt.Errorf("MVP 仅支持多头趋势回调分析")
	}

	limit := normalizeLimit(req.Limit, defaultTrendPullbackLimit, maxTrendPullbackLimit)
	stepLimit := normalizeLimit(req.StepLimit, defaultTrendPullbackStepLimit, maxTrendPullbackStepLimit)

	klines, err := a.klineRepo.GetLatestN(req.SymbolID, req.Period, limit)
	if err != nil {
		return nil, fmt.Errorf("获取K线失败: %w", err)
	}
	if len(klines) < minTrendPullbackContext+1 {
		return nil, fmt.Errorf("K线数量不足，至少需要 %d 根，当前 %d 根", minTrendPullbackContext+1, len(klines))
	}

	sort.Slice(klines, func(i, j int) bool {
		return klines[i].OpenTime.Before(klines[j].OpenTime)
	})

	start := len(klines) - stepLimit
	if start < minTrendPullbackContext {
		start = minTrendPullbackContext
	}

	resp := &TrendPullbackResponse{
		SymbolCode: req.SymbolCode,
		MarketCode: req.MarketCode,
		Period:     req.Period,
		Direction:  req.Direction,
		Steps:      make([]TrendPullbackStepResult, 0, len(klines)-start),
	}

	steps, err := a.analyzeBatch(ctx, req, klines, start)
	if err != nil {
		return nil, err
	}
	resp.Steps = steps
	resp.Analyzed = len(steps)

	for i := range resp.Steps {
		step := &resp.Steps[i]
		if isActionableBuyPoint(step) {
			resp.Found = true
			resp.Best = step
			break
		}
	}

	if resp.Found && req.SendFeishu && a.notifier != nil {
		a.notifier.SendToAll(a.buildNotification(req, resp.Best))
		resp.Notified = true
	}

	return resp, nil
}

func (a *TrendPullbackAnalyzer) analyzeBatch(ctx context.Context, req TrendPullbackRequest, klines []models.Kline, start int) ([]TrendPullbackStepResult, error) {
	messages := []ChatMessage{
		{Role: "system", Content: trendPullbackSystemPrompt()},
		{Role: "user", Content: buildTrendPullbackUserPrompt(req, klines, start)},
	}

	raw, err := a.client.ChatCompletionWithMaxTokens(ctx, messages, trendPullbackMaxOutputTokens)
	if err != nil {
		return nil, fmt.Errorf("AI 趋势回调分析失败: %w", err)
	}

	parsed, err := parseTrendPullbackBatchResponse(raw)
	if err != nil {
		a.logger.Warn("AI 趋势回调未返回结构化 JSON",
			zap.String("symbol_code", req.SymbolCode),
			zap.String("period", req.Period),
			zap.Error(err))
		return fallbackTrendPullbackSteps(klines, start, raw), nil
	}

	results := make([]TrendPullbackStepResult, 0, len(parsed.Steps))
	for _, step := range parsed.Steps {
		if step.KlineIndex < start || step.KlineIndex >= len(klines) {
			continue
		}
		current := klines[step.KlineIndex]
		buyPoint := normalizeTrendPullbackBuyPoint(step.BuyPoint)
		entryPrice := normalizeOptionalPrice(step.EntryPrice)
		stopLoss := normalizeOptionalPrice(step.StopLoss)
		takeProfit := normalizeOptionalPrice(step.TakeProfit)
		invalidationLevel := normalizeOptionalPrice(step.InvalidationLevel)
		if buyPoint != "ready" {
			entryPrice = nil
			stopLoss = nil
			takeProfit = nil
			invalidationLevel = nil
		}
		if buyPoint == "ready" && (entryPrice == nil || stopLoss == nil) {
			buyPoint = "watch"
			entryPrice = nil
			stopLoss = nil
			takeProfit = nil
			invalidationLevel = nil
		}
		decision := normalizeTrendPullbackDecision(step.Decision, buyPoint, step.TrendState, step.PullbackState)
		if decision == "alert" && buyPoint != "ready" {
			decision = "wait"
		}
		missedKlineIndex := normalizeMissedKlineIndex(step.Missed, step.MissedKlineIndex, start, len(klines))
		results = append(results, TrendPullbackStepResult{
			KlineTime:         current.OpenTime.UnixMilli(),
			ClosePrice:        current.ClosePrice,
			TrendState:        step.TrendState,
			PullbackState:     step.PullbackState,
			BuyPoint:          buyPoint,
			Decision:          decision,
			EntryPrice:        entryPrice,
			StopLoss:          stopLoss,
			TakeProfit:        takeProfit,
			InvalidationLevel: invalidationLevel,
			Confidence:        normalizeTrendPullbackConfidence(buyPoint, step.Confidence),
			Missed:            missedKlineIndex != nil,
			MissedKlineIndex:  missedKlineIndex,
			Reasoning:         step.Reasoning,
			RiskNotes:         step.RiskNotes,
		})
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("AI 未返回有效的观察K线结果")
	}

	return results, nil
}

func trendPullbackSystemPrompt() string {
	return `你是 smallfire 趋势交易系统里的“顺大逆小”回调买点分析器。只输出一个JSON对象，不要输出Markdown，不要输出解释性正文。

硬性输出要求：
- 禁止输出 <think>、分析过程、段落解释
- 第一字符必须是 {，最后字符必须是 }
- 如果无法判断，也必须输出 steps 数组，buy_point 使用 none 或 watch
- 不允许只输出思考过程后停止

交易系统：
- 只做已经确立的多头趋势，不预测底部，不抢反转
- 核心目标是在强趋势后的第一次健康回调里找买点
- 顺大：EMA30/EMA60/EMA90 多头排列、价格结构高低点抬高、上涨不是单根孤立暴拉
- 逆小：等待趋势内回调到 EMA30/EMA60/前高突破位附近，不追高
- 回调健康：回调幅度约为上一段上涨的 0.236-0.618，没有跌破关键结构低点，成交量缩小或波动收敛
- 回调危险：跌破前低、放量长阴、连续失守 EMA30/EMA60、回调超过 0.618
- 入场触发：支撑收回、阳包阴、Pin Bar/锤子线、假跌破后收回、突破小级别回调高点、放量反包
- 风控要求：必须能给出清楚止损位，风险收益比至少约 1.8，止损空间不能过大
- 回调买点不是突破前高后的追高确认；如果已经在 EMA30/EMA60/结构位附近出现支撑收回或反包，且止损和收益比成立，可以给 ready
- 不要把“突破前高才安全”作为 ready 的必要条件；突破前高更多用于止盈目标或加仓确认
- 如果后面的K线让你判断“最佳买点已过”，则必须把刚确认买点的那一根K线标为 ready；不能全程没有 ready 却事后说买点已过

你的任务：
用户手动选择一个可能有多头趋势的币对。输入包含一段历史上下文K线，以及最后若干根 observation=true 的观察K线。你要按时间顺序回放观察K线，对每根 observation=true 的K线判断：
1. 多头趋势是否仍然成立
2. 当前是否进入健康回调
3. 当前是否出现可执行的回调买点

严格禁止：
- 不要因为价格上涨就追高
- 不要在趋势衰竭、末端加速时给买点
- 不要只因为价格碰到 EMA 或支撑就给 ready，必须出现入场触发
- 不要把过深回调、放量长阴、结构破坏判断成健康回调
- 不要输出自由格式文本

输出JSON格式：
{
  "steps": [
    {
      "kline_index": 0,
      "trend_state": "confirmed|weak|exhaustion|unclear",
      "pullback_state": "none|started|healthy|dangerous|completed",
      "buy_point": "none|watch|ready",
      "decision": "wait|alert|invalid",
      "entry_price": 0,
      "stop_loss": 0,
      "take_profit": 0,
      "invalidation_level": 0,
      "confidence": 0,
      "missed": false,
      "missed_kline_index": 0,
      "reasoning": "40字以内中文理由",
      "risk_notes": ["风险1","风险2"]
    }
  ]
}

判定规则：
- buy_point=ready 只有在趋势确认、回调健康、出现支撑收回/反包/小结构突破、且止损位清楚时才允许
- 如果已出现支撑收回/阳包阴/假跌破收回，且止损可放在回调低点或关键 EMA 下方，必须评估 ready，不要只给 watch
- 如果只是接近回调区但没有触发，buy_point=watch
- 如果价格离EMA太远或放量加速，trend_state=exhaustion 或 buy_point=none
- confidence 表示“当前K线作为可执行买点”的置信度；buy_point=none 时为0，watch 通常为30-69，ready 必须为70-100
- confidence低于70时不要给 ready
- entry_price、stop_loss、take_profit、invalidation_level 只有 buy_point=ready 时才填写非0值；none/watch 一律填0
- 如果 pullback_state=completed 但 buy_point=none，表示这次回调没有有效买点，不要在 reasoning 中写“买点已过”
- decision=alert 必须等价于 buy_point=ready，并且必须有 entry_price、stop_loss、take_profit
- decision=wait 表示继续观察；decision=invalid 表示趋势衰竭、结构破坏或回调危险，当前跟踪失效
- 每根K线只能基于“本根收盘以及之前的数据”做当下决策，不要用后续K线回头改写前面判断
- missed=true 只允许表达复盘上的错过；如果 missed=true，missed_kline_index 必须填写这组 observation=true 中本应 alert 的 index
- 如果你无法指出 missed_kline_index，就不要写 missed=true，也不要在 reasoning 里说“已错过/买点已过”
- reasoning 中引用 EMA、价格、成交量等数值时，只能使用输入K线里真实出现的数值；不确定就不要写具体数值
- steps 必须只包含 observation=true 的K线，且 kline_index 必须使用输入中的 index`
}

func buildTrendPullbackUserPrompt(req TrendPullbackRequest, klines []models.Kline, observationStart int) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("标的: %s\n市场: %s\n周期: %s\n方向: %s\n", req.SymbolCode, req.MarketCode, req.Period, req.Direction))
	b.WriteString("K线按时间正序排列。observation=false 是趋势背景上下文；observation=true 是需要你逐根回放判断的观察K线。\n")
	b.WriteString("字段: index observation time open high low close volume ema30 ema60 ema90\n\n")

	start := len(klines) - 60
	if start < 0 {
		start = 0
	}
	for i := start; i < len(klines); i++ {
		k := klines[i]
		b.WriteString(fmt.Sprintf("%d %t %s %.6g %.6g %.6g %.6g %.0f %s %s %s\n",
			i,
			i >= observationStart,
			k.OpenTime.Format("01-02 15:04"),
			k.OpenPrice,
			k.HighPrice,
			k.LowPrice,
			k.ClosePrice,
			k.Volume,
			formatOptionalFloat(k.EMAShort),
			formatOptionalFloat(k.EMAMedium),
			formatOptionalFloat(k.EMALong),
		))
	}

	b.WriteString("\n请按时间顺序只分析 observation=true 的K线，并返回 steps 数组。")
	return b.String()
}

func (a *TrendPullbackAnalyzer) buildNotification(req TrendPullbackRequest, step *TrendPullbackStepResult) *notification.NotifyContent {
	message := fmt.Sprintf("币对: %s\n周期: %s\n方向: 做多\n状态: %s / %s\n置信度: %d\n理由: %s",
		req.SymbolCode, req.Period, step.TrendState, step.PullbackState, step.Confidence, step.Reasoning)
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
		message += "\n风险提示: " + strings.Join(step.RiskNotes, "；")
	}

	return &notification.NotifyContent{
		Title:   fmt.Sprintf("趋势回调买点提醒 %s", req.SymbolCode),
		Type:    "opportunity",
		Message: message,
		Data: map[string]interface{}{
			"period":     req.Period,
			"confidence": step.Confidence,
			"kline_time": time.UnixMilli(step.KlineTime).In(time.FixedZone("UTC+8", 8*60*60)).Format("2006-01-02 15:04:05"),
		},
	}
}

func isActionableBuyPoint(step *TrendPullbackStepResult) bool {
	return step != nil &&
		step.Decision == "alert" &&
		step.TrendState == "confirmed" &&
		(step.PullbackState == "healthy" || step.PullbackState == "completed") &&
		step.BuyPoint == "ready" &&
		step.Confidence >= 70 &&
		step.EntryPrice != nil &&
		step.StopLoss != nil
}

func parseTrendPullbackBatchResponse(raw string) (*trendPullbackBatchAIResult, error) {
	candidates := extractJSONCandidates(raw)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("未找到JSON对象")
	}

	var lastErr error
	for i := len(candidates) - 1; i >= 0; i-- {
		var result trendPullbackBatchAIResult
		if err := json.Unmarshal([]byte(candidates[i]), &result); err != nil {
			lastErr = err
			continue
		}
		if len(result.Steps) == 0 {
			lastErr = fmt.Errorf("steps 不能为空")
			continue
		}
		return &result, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("未找到包含 steps 的JSON对象")
}

func fallbackTrendPullbackSteps(klines []models.Kline, start int, raw string) []TrendPullbackStepResult {
	results := make([]TrendPullbackStepResult, 0, len(klines)-start)
	for i := start; i < len(klines); i++ {
		k := klines[i]
		results = append(results, TrendPullbackStepResult{
			KlineTime:     k.OpenTime.UnixMilli(),
			ClosePrice:    k.ClosePrice,
			TrendState:    "unclear",
			PullbackState: "none",
			BuyPoint:      "none",
			Decision:      "invalid",
			Confidence:    0,
			Missed:        false,
			Reasoning:     "AI未返回结构化JSON",
			RiskNotes:     []string{"模型输出被思考过程占满，未形成可执行判断"},
		})
	}
	return results
}

func normalizeConfidence(value int) int {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func normalizeTrendPullbackBuyPoint(value string) string {
	switch value {
	case "ready", "watch", "none":
		return value
	default:
		return "none"
	}
}

func normalizeTrendPullbackConfidence(buyPoint string, value int) int {
	confidence := normalizeConfidence(value)
	switch buyPoint {
	case "ready":
		if confidence < 70 {
			return 70
		}
		return confidence
	case "watch":
		if confidence < 30 {
			return 50
		}
		if confidence > 69 {
			return 69
		}
		return confidence
	default:
		return 0
	}
}

func normalizeTrendPullbackDecision(decision, buyPoint, trendState, pullbackState string) string {
	if buyPoint == "ready" {
		return "alert"
	}
	if trendState == "exhaustion" || pullbackState == "dangerous" {
		return "invalid"
	}
	switch decision {
	case "alert", "wait", "invalid":
	default:
		decision = ""
	}
	if decision != "" {
		return decision
	}
	return "wait"
}

func normalizeMissedKlineIndex(missed bool, missedKlineIndex *int, start, length int) *int {
	if !missed || missedKlineIndex == nil {
		return nil
	}
	if *missedKlineIndex < start || *missedKlineIndex >= length {
		return nil
	}
	value := *missedKlineIndex
	return &value
}

func normalizeOptionalPrice(value *float64) *float64 {
	if value == nil || *value <= 0 {
		return nil
	}
	return value
}

func normalizeLimit(value, fallback, max int) int {
	if value <= 0 {
		return fallback
	}
	if value > max {
		return max
	}
	return value
}

func formatOptionalFloat(v *float64) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%.6g", *v)
}
