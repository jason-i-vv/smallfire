package ai

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/smallfire/starfire/internal/models"
)

// TrendPullbackSkill 趋势回调策略
// 融合 trade-skills/trade-check SKILL.md 的"顺大逆小"核心策略
type TrendPullbackSkill struct{}

func (s *TrendPullbackSkill) Name() string { return "trend_pullback" }

func (s *TrendPullbackSkill) Description() string {
	return "趋势回调买点分析 — 顺大逆小策略，在强趋势后的健康回调中寻找买点"
}

func (s *TrendPullbackSkill) SystemPrompt(marketCode string) string {
	return `你是 smallfire 趋势交易系统的"顺大逆小"回调买点分析器。

## 核心理念
- **顺大** — 只做已经确立的多头趋势，不预测底部，不抢反转
- **逆小** — 等待趋势内回调到 EMA30/EMA60/前高突破位附近，不追高
- **三合一** — 趋势 + 形态 + 信号 必须同时成立才入场

## 分析框架

### 1. 趋势判断 (Trend)
- confirmed: EMA30/EMA60/EMA90 多头排列、价格结构高低点抬高、上涨不是单根孤立暴拉
- exhaustion: 末端加速、抛物线
- weak: 趋势转弱但仍未完全破坏
- unclear: 无明确趋势

### 2. 回调形态 (Formation)
- healthy: 回调幅度约上一段上涨的 0.236-0.618，未跌破关键结构低点，成交量缩小
- dangerous: 跌破前低、放量长阴、连续失守 EMA30/EMA60
- completed: 回调结束，出现止跌信号

### 3. 触发信号 (Signal)
- 支撑收回: 价格触及 EMA/支撑后收回
- 阳包阴: 当前阳线完全覆盖前一根阴线
- Pin Bar/锤子线: 下影线长于实体 2 倍以上
- 假跌破收回: 跌破 EMA30 后 1-2 根 K 线内收回（这是经典买点信号！）
- 突破小级别回调高点
- 放量反包

### 4. 风控 (Risk)
- 止损位必须清楚，放在回调低点或关键 EMA 下方
- 风险收益比至少 1.8
- 止损空间不能过大

## 关键规则（基于实战教训）

### 假跌破处理（最重要！）
- EMA30 下方出现深 wick 但随后 1-2 根 K 线内收回 = **假跌破**，这是经典买点，不是失效
- 不要仅因为单根 K 线的最低价跌破 EMA30 就判定趋势失效
- 必须观察收盘价是否站回 EMA30 上方

### 失效判断
- decision=invalid 需要谨慎：必须看到**明确的趋势结构破坏**
- 以下情况才判定 invalid:
  - 连续 2 根以上 K 线收盘价低于 EMA30，且无反弹迹象
  - 跌破关键结构低点（前一波上涨的起点）
  - 放量长阴跌破 EMA60
- 单根深 wick 后的收盘价仍在 EMA30 附近 = **cooldown**（观察），不是 invalid

### cooldown 状态
- 当价格暂时跌破 EMA30 但未确认趋势破坏时，使用 cooldown
- cooldown 意味着"暂时观望，等待下一根 K 线确认"
- 如果下一根 K 线收回 EMA30 上方，可以恢复 wait/watch

### 不要遗漏买点
- 如果后面的 K 线让你判断"最佳买点已过"，必须把确认买点的那根标为 ready
- 不能全程没有 ready 却事后说买点已过
- 如果已出现支撑收回/阳包阴/假跌破收回，且止损可放在回调低点下方，必须评估 ready

## 输出格式

只输出一个 JSON 对象，不要输出 Markdown，不要解释。

` + "```" + `json
{
  "steps": [
    {
      "kline_index": 0,
      "trend_state": "confirmed|weak|exhaustion|unclear",
      "pullback_state": "none|started|healthy|dangerous|completed",
      "buy_point": "none|watch|ready",
      "decision": "wait|alert|invalid|cooldown",
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
` + "```" + `

## 判定规则

- buy_point=ready: 趋势确认 + 回调健康 + 出现入场触发信号 + 止损位清楚 + confidence>=70
- buy_point=watch: 只是接近回调区但没有触发
- buy_point=none: 价格离 EMA 太远或放量加速
- decision=alert: 等价于 buy_point=ready，必须有 entry_price、stop_loss、take_profit
- decision=wait: 继续观察
- decision=cooldown: 暂时不确定（如单根深 wick），等待确认
- decision=invalid: 趋势结构明确破坏
- confidence: buy_point=none 时为 0，watch 为 30-69，ready 必须为 70-100
- entry_price、stop_loss、take_profit 只有 buy_point=ready 时填写，其他填 0
- 每根 K 线只基于"本根收盘以及之前的数据"做当下决策
- reasoning 中引用数值时，只能使用输入 K 线里真实出现的数值`
}

func (s *TrendPullbackSkill) BuildFirstMessage(klines []models.Kline, observationStart int) string {
	var b strings.Builder
	b.WriteString("首次分析。K线按时间正序排列。observation=false 是趋势背景上下文；observation=true 是需要逐根回放判断的观察K线。\n")
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
			k.OpenPrice, k.HighPrice, k.LowPrice, k.ClosePrice,
			k.Volume,
			formatOptionalFloat(k.EMAShort),
			formatOptionalFloat(k.EMAMedium),
			formatOptionalFloat(k.EMALong),
		))
	}

	b.WriteString("\n请按时间顺序只分析 observation=true 的K线，并返回 steps 数组。")
	return b.String()
}

func (s *TrendPullbackSkill) BuildIncrementalMessage(klines []models.Kline) string {
	var b strings.Builder
	b.WriteString("新 K 线到达，请基于之前的分析继续判断：\n")
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

	b.WriteString("\n请分析这些新 K 线，基于之前的上下文继续判断趋势状态和买点。")
	return b.String()
}

// skillPullbackAIStep AI 返回的单步结果
type skillPullbackAIStep struct {
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

type skillPullbackAIResult struct {
	Steps []skillPullbackAIStep `json:"steps"`
}

func (s *TrendPullbackSkill) ParseResponse(raw string) ([]AnalysisStep, error) {
	candidates := extractJSONCandidates(raw)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("未找到 JSON 对象")
	}

	var lastErr error
	for i := len(candidates) - 1; i >= 0; i-- {
		var result skillPullbackAIResult
		if err := json.Unmarshal([]byte(candidates[i]), &result); err != nil {
			lastErr = err
			continue
		}
		if len(result.Steps) == 0 {
			lastErr = fmt.Errorf("steps 不能为空")
			continue
		}

		steps := make([]AnalysisStep, 0, len(result.Steps))
		for _, step := range result.Steps {
			buyPoint := normalizeTrendPullbackBuyPoint(step.BuyPoint)
			entryPrice := normalizeOptionalPrice(step.EntryPrice)
			stopLoss := normalizeOptionalPrice(step.StopLoss)
			takeProfit := normalizeOptionalPrice(step.TakeProfit)

			if buyPoint != "ready" {
				entryPrice = nil
				stopLoss = nil
				takeProfit = nil
			}
			if buyPoint == "ready" && (entryPrice == nil || stopLoss == nil) {
				buyPoint = "watch"
				entryPrice = nil
				stopLoss = nil
				takeProfit = nil
			}

			decision := s.normalizeDecision(step.Decision, buyPoint, step.TrendState, step.PullbackState)

			steps = append(steps, AnalysisStep{
				TrendState:    step.TrendState,
				PullbackState: step.PullbackState,
				BuyPoint:      buyPoint,
				Decision:      decision,
				EntryPrice:    entryPrice,
				StopLoss:      stopLoss,
				TakeProfit:    takeProfit,
				Confidence:    normalizeTrendPullbackConfidence(buyPoint, step.Confidence),
				Reasoning:     step.Reasoning,
				RiskNotes:     step.RiskNotes,
			})
		}
		return steps, nil
	}

	return nil, lastErr
}

// normalizeDecision 改进版：dangerous 回调不再自动 invalid，而是 cooldown
func (s *TrendPullbackSkill) normalizeDecision(decision, buyPoint, trendState, pullbackState string) string {
	if buyPoint == "ready" {
		return "alert"
	}
	if trendState == "exhaustion" {
		return "invalid"
	}
	// dangerous 回调 → cooldown（而非直接 invalid）
	if pullbackState == "dangerous" {
		switch decision {
		case "invalid":
			return "invalid"
		default:
			return "cooldown"
		}
	}
	switch decision {
	case "alert", "wait", "invalid", "cooldown":
	default:
		decision = ""
	}
	if decision != "" {
		return decision
	}
	return "wait"
}
