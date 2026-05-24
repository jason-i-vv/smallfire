package ai

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/smallfire/starfire/internal/models"
)

// TimePriceProjectionSkill 时间价格等距策略
// 基于 A-B-C 波段结构，寻找顺趋势回调/反弹后的 1:1 等距投射机会。
type TimePriceProjectionSkill struct{}

func (s *TimePriceProjectionSkill) Name() string { return "time_price_projection" }

func (s *TimePriceProjectionSkill) Description() string {
	return "时间价格等距 — 基于 A-B-C 波段结构寻找 1:1 等距目标的顺趋势机会"
}

func (s *TimePriceProjectionSkill) SystemPrompt(marketCode string) string {
	return `你是 smallfire 趋势交易系统的"时间价格等距"狙击分析器。

## 核心理念
- 先读取用户消息里的「方向」字段：做多寻找 A-B-C 上涨等距机会；做空寻找 A-B-C 下跌等距机会
- 只做顺趋势结构，不抢反转；信号来自价格波段和时间节奏，不是单纯 EMA 回踩
- 目标价固定使用 1:1 等距投射；止损固定使用 C 点后的波段低点/高点

## 做多结构
1. A 点: 前方明确波段低点
2. B 点: A 后明确推动高点，形成 AB 上涨段
3. C 点: B 后回调低点，不能跌破 A；最好靠近 EMA30/EMA60/前突破位
4. 入场: C 后出现止跌转强、突破小级别回调高点、放量阳线或重新站回均线
5. 止损: C 点或 C 后最近波段低点下方
6. 止盈: C + (B - A)，即 AB 段从 C 点向上 1:1 等距投射

## 做空结构
1. A 点: 前方明确波段高点
2. B 点: A 后明确推动低点，形成 AB 下跌段
3. C 点: B 后反弹高点，不能突破 A；最好靠近 EMA30/EMA60/前跌破位
4. 入场: C 后出现承压转弱、跌破小级别反弹低点、放量阴线或重新跌回均线
5. 止损: C 点或 C 后最近波段高点上方
6. 止盈: C - (A - B)，即 AB 段从 C 点向下 1:1 等距投射

## 时间和价格要求
- AB 必须是一段清楚推动，不是杂乱横盘
- BC 必须是正常回调/反弹，不能破坏 A 点
- C 点之后触发不能太晚；如果已经走到等距目标附近，不再给 ready
- 时间节奏越接近越好：BC 的耗时通常不应远大于 AB 的 2 倍；明显拖太久只能 watch
- 如果 A/B/C 点不清楚，不能 ready

## ready 硬检查
只有全部满足才允许 buy_point=ready / decision=alert：
1. trend_state 必须是 confirmed
2. projection_state 必须是 ready
3. A/B/C 三点必须清楚，且 C 未破坏 A
4. 做多必须 stop_loss < entry_price < take_profit
5. 做空必须 take_profit < entry_price < stop_loss
6. take_profit 必须接近 1:1 等距目标；不能让 AI 自由估目标
7. 同一段 ABC 只标记第一次可执行触发，后续确认不要重复 ready

## 失效判断
- 做多：收盘跌破 C 点或前方波段低点，结构失效
- 做空：收盘突破 C 点或前方波段高点，结构失效
- 仅跌破/站上 EMA 不等于失效，优先 cooldown

## 输出格式
只输出一个 JSON 对象，不要输出 Markdown，不要解释。

` + "```" + `json
{
  "steps": [
    {
      "kline_index": 0,
      "trend_state": "confirmed|weak|exhaustion|unclear",
      "projection_state": "none|forming|ready|triggered|invalid",
      "buy_point": "none|watch|ready",
      "decision": "wait|alert|invalid|cooldown",
      "entry_price": 0,
      "stop_loss": 0,
      "take_profit": 0,
      "swing_a": 0,
      "swing_b": 0,
      "swing_c": 0,
      "projection_target": 0,
      "stop_level": 0,
      "ab_range": 0,
      "time_symmetry": "matched|early|late|unclear",
      "confidence": 0,
      "reasoning": "40字以内中文理由",
      "risk_notes": ["风险1","风险2"]
    }
  ]
}
` + "```" + `

## 判定规则
- buy_point=ready: A/B/C 清楚 + 顺趋势 + C 后触发 + 止损在 C/波段位 + 止盈为 1:1 等距目标 + confidence>=70
- buy_point=watch: ABC 正在形成或 C 后还没有触发
- buy_point=none: 无清楚 ABC、已接近目标、或价格结构杂乱
- decision=alert 等价于 buy_point=ready，必须有 entry_price、stop_loss、take_profit
- decision=invalid 只在 C 点/前波段结构被明确收盘破坏时使用
- confidence: none 为 0，watch 为 30-69，ready 为 70-100
- 每根 K 线只基于本根收盘及之前的数据判断
- reasoning 中引用价格/成交量/EMA，只能使用输入 K 线真实数值`
}

func (s *TimePriceProjectionSkill) BuildFirstMessage(klines []models.Kline, observationStart int) string {
	var b strings.Builder
	b.WriteString("首次分析。K线按时间正序排列。observation=false 是趋势背景上下文；observation=true 是需要逐根回放判断的观察K线。\n")
	b.WriteString("请寻找 A-B-C 时间价格等距结构，目标价使用 C 点 +/- AB 等距投射。\n")
	b.WriteString("字段: index observation time open high low close volume ema30 ema60 ema90\n\n")

	start := len(klines) - 80
	if start < 0 {
		start = 0
	}
	for i := start; i < len(klines); i++ {
		k := klines[i]
		b.WriteString(fmt.Sprintf("%d %t %s %.6g %.6g %.6g %.6g %.0f %s %s %s\n",
			i,
			i >= observationStart,
			k.OpenTime.Format("01-02 15:04"),
			k.OpenPrice, k.HighPrice, k.LowPrice, k.ClosePrice,
			k.Volume,
			formatOptionalFloat(k.EMAShort),
			formatOptionalFloat(k.EMAMedium),
			formatOptionalFloat(k.EMALong),
		))
	}

	b.WriteString("\n请按方向逐根分析 observation=true 的K线，并返回 steps 数组。")
	return b.String()
}

func (s *TimePriceProjectionSkill) BuildIncrementalMessage(klines []models.Kline) string {
	var b strings.Builder
	b.WriteString("新 K 线到达，请基于之前的 A-B-C 等距分析继续判断：\n")
	b.WriteString("字段: index time open high low close volume ema30 ema60 ema90\n\n")

	for i, k := range klines {
		b.WriteString(fmt.Sprintf("%d %s %.6g %.6g %.6g %.6g %.0f %s %s %s\n",
			i,
			k.OpenTime.Format("01-02 15:04"),
			k.OpenPrice, k.HighPrice, k.LowPrice, k.ClosePrice,
			k.Volume,
			formatOptionalFloat(k.EMAShort),
			formatOptionalFloat(k.EMAMedium),
			formatOptionalFloat(k.EMALong),
		))
	}

	b.WriteString("\n请继续判断是否形成或触发 1:1 等距入场机会。")
	return b.String()
}

type timePriceAIStep struct {
	KlineIndex       int      `json:"kline_index"`
	TrendState       string   `json:"trend_state"`
	ProjectionState  string   `json:"projection_state"`
	BuyPoint         string   `json:"buy_point"`
	Decision         string   `json:"decision"`
	EntryPrice       *float64 `json:"entry_price"`
	StopLoss         *float64 `json:"stop_loss"`
	TakeProfit       *float64 `json:"take_profit"`
	SwingA           *float64 `json:"swing_a"`
	SwingB           *float64 `json:"swing_b"`
	SwingC           *float64 `json:"swing_c"`
	ProjectionTarget *float64 `json:"projection_target"`
	StopLevel        *float64 `json:"stop_level"`
	ABRange          *float64 `json:"ab_range"`
	TimeSymmetry     string   `json:"time_symmetry"`
	Confidence       int      `json:"confidence"`
	Reasoning        string   `json:"reasoning"`
	RiskNotes        []string `json:"risk_notes"`
}

type timePriceAIResult struct {
	Steps []timePriceAIStep `json:"steps"`
}

func (s *TimePriceProjectionSkill) ParseResponse(raw string) ([]AnalysisStep, error) {
	candidates := extractJSONCandidates(raw)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("未找到 JSON 对象")
	}

	var lastErr error
	for i := len(candidates) - 1; i >= 0; i-- {
		var result timePriceAIResult
		if err := json.Unmarshal([]byte(candidates[i]), &result); err != nil {
			lastErr = err
			continue
		}
		if len(result.Steps) == 0 {
			lastErr = fmt.Errorf("steps 不能为空")
			continue
		}

		steps := make([]AnalysisStep, 0, len(result.Steps))
		alertOpen := false
		for _, step := range result.Steps {
			buyPoint := normalizeTrendPullbackBuyPoint(step.BuyPoint)
			entryPrice := normalizeOptionalPrice(step.EntryPrice)
			stopLoss := normalizeOptionalPrice(step.StopLoss)
			takeProfit := normalizeOptionalPrice(step.TakeProfit)
			riskNotes := append([]string(nil), step.RiskNotes...)

			if buyPoint != "ready" {
				entryPrice = nil
				stopLoss = nil
				takeProfit = nil
			}
			if buyPoint == "ready" && !isValidTimePriceReady(entryPrice, stopLoss, takeProfit) {
				buyPoint = "watch"
				entryPrice = nil
				stopLoss = nil
				takeProfit = nil
				riskNotes = appendTrendPullbackGateNote(riskNotes, "未通过时间价格等距硬校验：需入场、止损、1:1等距止盈方向正确")
			}
			if buyPoint == "ready" {
				if alertOpen {
					buyPoint = "watch"
					entryPrice = nil
					stopLoss = nil
					takeProfit = nil
					riskNotes = appendTrendPullbackGateNote(riskNotes, "同一段ABC已出现等距信号，避免重复提醒")
				} else {
					alertOpen = true
				}
			} else if step.ProjectionState == "forming" || step.ProjectionState == "none" {
				alertOpen = false
			}

			decision := normalizeTimePriceDecision(step.Decision, buyPoint)
			extra := buildTimePriceExtra(step)
			steps = append(steps, AnalysisStep{
				TrendState:    step.TrendState,
				PullbackState: normalizeProjectionState(step.ProjectionState),
				BuyPoint:      buyPoint,
				Decision:      decision,
				EntryPrice:    entryPrice,
				StopLoss:      stopLoss,
				TakeProfit:    takeProfit,
				Confidence:    normalizeTrendPullbackConfidence(buyPoint, step.Confidence),
				Reasoning:     step.Reasoning,
				RiskNotes:     riskNotes,
				Extra:         extra,
			})
		}
		return steps, nil
	}

	return nil, lastErr
}

func isValidTimePriceReady(entryPrice, stopLoss, takeProfit *float64) bool {
	if entryPrice == nil || stopLoss == nil || takeProfit == nil {
		return false
	}
	if *stopLoss < *entryPrice && *takeProfit > *entryPrice {
		return true
	}
	if *stopLoss > *entryPrice && *takeProfit < *entryPrice {
		return true
	}
	return false
}

func normalizeTimePriceDecision(decision, buyPoint string) string {
	if buyPoint == "ready" {
		return "alert"
	}
	switch decision {
	case "wait", "invalid", "cooldown":
		return decision
	default:
		return "wait"
	}
}

func normalizeProjectionState(state string) string {
	switch state {
	case "ready", "triggered":
		return "completed"
	case "forming":
		return "healthy"
	case "invalid":
		return "dangerous"
	default:
		return "none"
	}
}

func buildTimePriceExtra(step timePriceAIStep) map[string]interface{} {
	extra := map[string]interface{}{
		"strategy":         "time_price_projection",
		"projection_state": step.ProjectionState,
		"time_symmetry":    step.TimeSymmetry,
	}
	addOptionalExtra(extra, "swing_a", step.SwingA)
	addOptionalExtra(extra, "swing_b", step.SwingB)
	addOptionalExtra(extra, "swing_c", step.SwingC)
	addOptionalExtra(extra, "projection_target", step.ProjectionTarget)
	addOptionalExtra(extra, "stop_level", step.StopLevel)
	addOptionalExtra(extra, "ab_range", step.ABRange)
	return extra
}

func addOptionalExtra(extra map[string]interface{}, key string, value *float64) {
	if value != nil && *value > 0 {
		extra[key] = *value
	}
}
