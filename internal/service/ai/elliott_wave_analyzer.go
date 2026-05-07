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
	defaultElliottWaveLimit     = 160
	defaultElliottWaveStepLimit = 12
	maxElliottWaveLimit         = 240
	maxElliottWaveStepLimit     = 30
	minElliottWaveContext       = 50
	elliottWaveMaxOutputTokens  = 6000
)

// ElliottWaveAnalyzer 手动艾略特波浪 / 主升低吸 AI 分析器。
// 第一版用于快速验证：用户手动选择市场、标的和周期，系统批量回放 K 线寻找波浪买点。
type ElliottWaveAnalyzer struct {
	client    *AIClient
	klineRepo repository.KlineRepo
	notifier  *notification.Manager
	logger    *zap.Logger
}

type ElliottWaveRequest struct {
	SymbolID   int    `json:"symbol_id"`
	SymbolCode string `json:"symbol_code"`
	MarketCode string `json:"market_code"`
	Period     string `json:"period"`
	Limit      int    `json:"limit"`
	StepLimit  int    `json:"step_limit"`
	SendFeishu bool   `json:"send_feishu"`
}

type ElliottWaveResponse struct {
	SymbolCode string                  `json:"symbol_code"`
	MarketCode string                  `json:"market_code"`
	Period     string                  `json:"period"`
	Analyzed   int                     `json:"analyzed"`
	Found      bool                    `json:"found"`
	Best       *ElliottWaveStepResult  `json:"best,omitempty"`
	Steps      []ElliottWaveStepResult `json:"steps"`
	Notified   bool                    `json:"notified"`
}

type ElliottWaveStepResult struct {
	KlineTime         int64    `json:"kline_time"`
	ClosePrice        float64  `json:"close_price"`
	WaveStage         string   `json:"wave_stage"`
	PatternType       string   `json:"pattern_type"`
	SetupStatus       string   `json:"setup_status"`
	BuyPoint          string   `json:"buy_point"`
	EntryPrice        *float64 `json:"entry_price,omitempty"`
	StopLoss          *float64 `json:"stop_loss,omitempty"`
	TargetPrice       *float64 `json:"target_price,omitempty"`
	InvalidationLevel *float64 `json:"invalidation_level,omitempty"`
	Confidence        int      `json:"confidence"`
	Reasoning         string   `json:"reasoning"`
	WaveCount         string   `json:"wave_count"`
	RiskNotes         []string `json:"risk_notes"`
	Raw               string   `json:"raw,omitempty"`
}

type elliottWaveAIResult struct {
	KlineIndex        int      `json:"kline_index"`
	WaveStage         string   `json:"wave_stage"`
	PatternType       string   `json:"pattern_type"`
	SetupStatus       string   `json:"setup_status"`
	BuyPoint          string   `json:"buy_point"`
	EntryPrice        *float64 `json:"entry_price"`
	StopLoss          *float64 `json:"stop_loss"`
	TargetPrice       *float64 `json:"target_price"`
	InvalidationLevel *float64 `json:"invalidation_level"`
	Confidence        int      `json:"confidence"`
	Reasoning         string   `json:"reasoning"`
	WaveCount         string   `json:"wave_count"`
	RiskNotes         []string `json:"risk_notes"`
}

type elliottWaveBatchAIResult struct {
	Steps []elliottWaveAIResult `json:"steps"`
}

func NewElliottWaveAnalyzer(client *AIClient, klineRepo repository.KlineRepo, notifier *notification.Manager, logger *zap.Logger) *ElliottWaveAnalyzer {
	return &ElliottWaveAnalyzer{
		client:    client,
		klineRepo: klineRepo,
		notifier:  notifier,
		logger:    logger,
	}
}

func (a *ElliottWaveAnalyzer) Analyze(ctx context.Context, req ElliottWaveRequest) (*ElliottWaveResponse, error) {
	if a == nil || a.client == nil {
		return nil, fmt.Errorf("AI 分析服务未启用")
	}
	if req.SymbolID <= 0 {
		return nil, fmt.Errorf("symbol_id 不能为空")
	}
	if req.MarketCode == "" {
		req.MarketCode = "bybit"
	}
	if req.Period == "" {
		req.Period = defaultElliottPeriod(req.MarketCode)
	}

	limit := normalizeLimit(req.Limit, defaultElliottWaveLimit, maxElliottWaveLimit)
	stepLimit := normalizeLimit(req.StepLimit, defaultElliottWaveStepLimit, maxElliottWaveStepLimit)

	klines, err := a.klineRepo.GetLatestN(req.SymbolID, req.Period, limit)
	if err != nil {
		return nil, fmt.Errorf("获取K线失败: %w", err)
	}
	if len(klines) < minElliottWaveContext+1 {
		return nil, fmt.Errorf("K线数量不足，至少需要 %d 根，当前 %d 根", minElliottWaveContext+1, len(klines))
	}

	sort.Slice(klines, func(i, j int) bool {
		return klines[i].OpenTime.Before(klines[j].OpenTime)
	})

	start := len(klines) - stepLimit
	if start < minElliottWaveContext {
		start = minElliottWaveContext
	}

	steps, err := a.analyzeBatch(ctx, req, klines, start)
	if err != nil {
		return nil, err
	}

	resp := &ElliottWaveResponse{
		SymbolCode: req.SymbolCode,
		MarketCode: req.MarketCode,
		Period:     req.Period,
		Steps:      steps,
		Analyzed:   len(steps),
	}

	for i := range resp.Steps {
		step := &resp.Steps[i]
		if isActionableElliottBuyPoint(step) {
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

func (a *ElliottWaveAnalyzer) analyzeBatch(ctx context.Context, req ElliottWaveRequest, klines []models.Kline, start int) ([]ElliottWaveStepResult, error) {
	messages := []ChatMessage{
		{Role: "system", Content: elliottWaveSystemPrompt()},
		{Role: "user", Content: buildElliottWaveUserPrompt(req, klines, start)},
	}

	raw, err := a.client.ChatCompletionWithMaxTokens(ctx, messages, elliottWaveMaxOutputTokens)
	if err != nil {
		return nil, fmt.Errorf("AI 艾略特波浪分析失败: %w", err)
	}

	parsed, err := parseElliottWaveBatchResponse(raw)
	if err != nil {
		a.logger.Warn("AI 艾略特波浪未返回结构化 JSON",
			zap.String("symbol_code", req.SymbolCode),
			zap.String("market_code", req.MarketCode),
			zap.String("period", req.Period),
			zap.Error(err))
		return fallbackElliottWaveSteps(klines, start, raw), nil
	}

	results := make([]ElliottWaveStepResult, 0, len(parsed.Steps))
	for _, step := range parsed.Steps {
		if step.KlineIndex < start || step.KlineIndex >= len(klines) {
			continue
		}
		current := klines[step.KlineIndex]
		results = append(results, ElliottWaveStepResult{
			KlineTime:         current.OpenTime.UnixMilli(),
			ClosePrice:        current.ClosePrice,
			WaveStage:         step.WaveStage,
			PatternType:       step.PatternType,
			SetupStatus:       step.SetupStatus,
			BuyPoint:          step.BuyPoint,
			EntryPrice:        normalizeOptionalPrice(step.EntryPrice),
			StopLoss:          normalizeOptionalPrice(step.StopLoss),
			TargetPrice:       normalizeOptionalPrice(step.TargetPrice),
			InvalidationLevel: normalizeOptionalPrice(step.InvalidationLevel),
			Confidence:        normalizeConfidence(step.Confidence),
			Reasoning:         step.Reasoning,
			WaveCount:         step.WaveCount,
			RiskNotes:         step.RiskNotes,
		})
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("AI 未返回有效的观察K线结果")
	}

	return results, nil
}

func elliottWaveSystemPrompt() string {
	return `你是 smallfire 的艾略特波浪 / A股主升低吸买点分析器。只输出一个JSON对象，不要输出Markdown，不要输出解释性正文。

硬性输出要求：
- 禁止输出 <think>、分析过程、段落解释
- 第一字符必须是 {，最后字符必须是 }
- 如果无法判断，也必须输出 steps 数组，buy_point 使用 none 或 watch
- 不允许只输出思考过程后停止

通用艾略特规则：
- 先判断大方向，再找推动浪5浪和修正浪3浪，不要强行数浪
- 推动浪规则：2浪不能跌破1浪起点；3浪必须超过1浪终点；3浪不能是1/3/5中最短；非杠杆现货里4浪不能进入1浪价格区间
- 修正浪可为ABC、平台、三角形、组合修正；Wave 2 常回撤 50%-61.8%，Wave 4 常回撤约38.2%
- 优先寻找 Wave 2 或 Wave 4 修正结束后的低风险买点；不要在 Wave 5 末端或抛物线加速时追高
- 买点必须有清楚失效位，风险收益比应至少约1.8

A股主升低吸规则，market_code=a_stock 时优先级高于松散数浪：
- 必须从左到右识别结构：0点 -> 一高 -> 1点 -> 二高 -> 2点 -> 试盘线/主升段 -> 回踩操盘线买点
- 一高/二高按收盘价判断，二高不能高于一高；1点/2点按最低价判断，2点必须高于1点
- Type B：一高 -> 1点 -> 二高 -> 2点，干净主升结构
- Type A：一高 -> 二高 -> 1点 -> 2点，或一次重置后形成更高2点
- 买点不是二高突破本身，而是2点确认后出现试盘线/主升段，再第一次回踩操盘线附近并站稳
- 操盘线优先理解为MA20；若输入没有MA20，用ema30/ema_short近似，但必须在理由中说明是近似
- 不买：大阴线、连续阴线、不能收回操盘线、跌破2点、结构失败、市场/板块明显弱

输入包含一段历史上下文K线，以及最后若干根 observation=true 的观察K线。你要按时间顺序回放 observation=true 的K线，判断每根K线是否形成可跟踪或可执行买点。

输出JSON格式：
{
  "steps": [
    {
      "kline_index": 0,
      "wave_stage": "wave1|wave2|wave3|wave4|wave5|abc_correction|main_rise_low_buy|unclear|invalidated",
      "pattern_type": "impulse|correction|type_a|type_b|tracking|failed|unclear",
      "setup_status": "confirmed|tracking|warning|invalidated",
      "buy_point": "none|watch|ready",
      "entry_price": 0,
      "stop_loss": 0,
      "target_price": 0,
      "invalidation_level": 0,
      "confidence": 0,
      "reasoning": "60字以内中文理由",
      "wave_count": "简短波浪/主升结构标注",
      "risk_notes": ["风险1","风险2"]
    }
  ]
}

判定规则：
- buy_point=ready 只能用于结构 confirmed、出现明确低吸/修正结束/突破回踩触发、且止损位清楚时
- 只有 tracking 或结构未完成时，buy_point=watch
- Wave 5末端、抛物线加速、A股跌破2点、放量破操盘线，都必须 buy_point=none 或 setup_status=warning/invalidated
- confidence低于70时不要给 ready
- steps 必须只包含 observation=true 的K线，且 kline_index 必须使用输入中的 index`
}

func buildElliottWaveUserPrompt(req ElliottWaveRequest, klines []models.Kline, observationStart int) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("标的: %s\n市场: %s\n周期: %s\n", req.SymbolCode, req.MarketCode, req.Period))
	b.WriteString("K线按时间正序排列。observation=false 是结构背景上下文；observation=true 是需要逐根回放判断的观察K线。\n")
	b.WriteString("字段: index observation time open high low close volume ema_short ema_medium ema_long\n\n")

	start := len(klines) - 90
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

func (a *ElliottWaveAnalyzer) buildNotification(req ElliottWaveRequest, step *ElliottWaveStepResult) *notification.NotifyContent {
	message := fmt.Sprintf("标的: %s\n市场: %s\n周期: %s\n阶段: %s / %s\n买点: %s\n置信度: %d\n结构: %s\n理由: %s",
		req.SymbolCode, req.MarketCode, req.Period, step.WaveStage, step.PatternType, step.BuyPoint, step.Confidence, step.WaveCount, step.Reasoning)
	if step.EntryPrice != nil {
		message += fmt.Sprintf("\n建议入场: %.6g", *step.EntryPrice)
	}
	if step.StopLoss != nil {
		message += fmt.Sprintf("\n建议止损: %.6g", *step.StopLoss)
	}
	if step.TargetPrice != nil {
		message += fmt.Sprintf("\n目标价: %.6g", *step.TargetPrice)
	}
	if len(step.RiskNotes) > 0 {
		message += "\n风险提示: " + strings.Join(step.RiskNotes, "；")
	}

	return &notification.NotifyContent{
		Title:   fmt.Sprintf("艾略特波浪买点提醒 %s", req.SymbolCode),
		Type:    "opportunity",
		Message: message,
		Data: map[string]interface{}{
			"period":     req.Period,
			"confidence": step.Confidence,
			"kline_time": time.UnixMilli(step.KlineTime).In(time.FixedZone("UTC+8", 8*60*60)).Format("2006-01-02 15:04:05"),
		},
	}
}

func isActionableElliottBuyPoint(step *ElliottWaveStepResult) bool {
	return step != nil &&
		step.SetupStatus == "confirmed" &&
		step.BuyPoint == "ready" &&
		step.Confidence >= 70 &&
		step.EntryPrice != nil &&
		step.StopLoss != nil
}

func parseElliottWaveBatchResponse(raw string) (*elliottWaveBatchAIResult, error) {
	candidates := extractJSONCandidates(raw)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("未找到JSON对象")
	}

	var lastErr error
	for i := len(candidates) - 1; i >= 0; i-- {
		var result elliottWaveBatchAIResult
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

func fallbackElliottWaveSteps(klines []models.Kline, start int, raw string) []ElliottWaveStepResult {
	results := make([]ElliottWaveStepResult, 0, len(klines)-start)
	for i := start; i < len(klines); i++ {
		k := klines[i]
		results = append(results, ElliottWaveStepResult{
			KlineTime:   k.OpenTime.UnixMilli(),
			ClosePrice:  k.ClosePrice,
			WaveStage:   "unclear",
			PatternType: "unclear",
			SetupStatus: "warning",
			BuyPoint:    "none",
			Confidence:  0,
			Reasoning:   "AI未返回结构化JSON",
			WaveCount:   "未形成有效波浪标注",
			RiskNotes:   []string{"模型输出被思考过程占满，未形成可执行判断"},
		})
	}
	return results
}

func defaultElliottPeriod(marketCode string) string {
	if marketCode == "a_stock" {
		return models.Period1d
	}
	return models.Period1h
}
