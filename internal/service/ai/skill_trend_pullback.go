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
	return "趋势回调买点/空点分析 — 顺大逆小策略，在强趋势后的健康回调/反弹中寻找可执行信号"
}

func (s *TrendPullbackSkill) SystemPrompt(marketCode string) string {
	return `你是 smallfire 趋势交易系统的"顺大逆小"回调买点分析器。

## 核心理念
- 先读取用户消息里的「方向」字段：做多按多头回调买点判断；做空按空头反弹空点判断
- **顺大** — 只做已经确立的趋势，不预测底部/顶部，不抢反转
- **逆小** — 做多等待趋势内回调到 EMA30/EMA60/前高突破位附近；做空等待趋势内反弹到 EMA30/EMA60/前低跌破位附近
- **三合一** — 趋势 + 形态 + 信号 必须同时成立才入场

## 分析框架

### 1. 趋势判断 (Trend)
- confirmed:
  - 做多: EMA30/EMA60/EMA90 多头排列并有斜率，价格结构高低点抬高，价格主要运行在 EMA30/EMA60 上方，上涨不是单根孤立暴拉
  - 做空: EMA30/EMA60/EMA90 空头排列并有斜率，价格结构高低点下移，价格主要运行在 EMA30/EMA60 下方，下跌不是单根孤立暴跌
- exhaustion: 末端加速、抛物线、过热；表示不追价，继续等待新回调，不等于趋势失效
- weak: 趋势转弱但仍未完全破坏
- unclear: 无明确趋势

### 2. 回调形态 (Formation)
- healthy:
  - 做多: 已从近期高点明显回落，回调幅度约上一段上涨的 0.236-0.618，靠近 EMA30/EMA60/前高突破位，未跌破关键结构低点，成交量缩小或波动收敛
  - 做空: 已从近期低点明显反弹，反弹幅度约上一段下跌的 0.236-0.618，靠近 EMA30/EMA60/前低跌破位，未突破关键结构高点，成交量缩小或波动收敛
- dangerous:
  - 做多: 跌破前低、放量长阴、连续失守 EMA30/EMA60
  - 做空: 突破前高、放量长阳、连续收回 EMA30/EMA60 上方
- completed: 回调结束，出现止跌信号

### 3. 触发信号 (Signal)
- 做多触发: 支撑收回、下影 Pin Bar/锤子线、假跌破 EMA30 后收回、突破小级别回调高点、放量阳线反包
- 做空触发: 压力回落、上影 Pin Bar/倒锤线、假突破 EMA30 后跌回、跌破小级别反弹低点、放量阴线反包

### 4. 风控 (Risk)
- 止损位必须清楚；做多放在回调低点或关键 EMA 下方，做空放在反弹高点或关键 EMA 上方
- 止盈位必须清楚；做多止盈要高于入场，做空止盈要低于入场
- 风险收益比至少 1.8。做多按 (take_profit-entry_price)/(entry_price-stop_loss) 计算；做空按 (entry_price-take_profit)/(stop_loss-entry_price) 计算
- 止损空间不能过大

### 5. ready 硬检查清单
只有全部满足时才允许 buy_point=ready / decision=alert：
1. trend_state 必须是 confirmed
2. pullback_state 必须是 healthy 或 completed
3. 当前 K 线必须是回调/反弹后的第一类触发信号，而不是已经续涨/续跌后的追价确认
4. 做多必须满足 stop_loss < entry_price < take_profit；做空必须满足 take_profit < entry_price < stop_loss
5. 风险收益比必须 >= 1.8
6. 同一段回调/反弹只能标记第一次可执行触发，后续 K 线即使继续走强/走弱，也只能 watch 或 none

## 关键规则（基于实战教训）

### 不是买点/空点的情况
- 做多时，价格已接近前高、刚突破前高、或离 EMA30/EMA60 很远，只能 watch/none，不能 ready
- 做空时，价格已接近前低、刚跌破前低、或离 EMA30/EMA60 很远，只能 watch/none，不能 ready
- 只有趋势延续、阳线续涨、阴线续跌、高位/低位横盘，没有回调完成触发，不能 ready
- 如果 take_profit 离 entry_price 很近、stop_loss 很远导致收益风险比不足，不能 ready

### 假突破/假跌破处理（最重要！）
- EMA30 下方出现深 wick 但随后 1-2 根 K 线内收回 = **假跌破**，这是经典买点，不是失效
- EMA30 上方出现长上影但随后 1-2 根 K 线内跌回 = **假突破**，这是经典空点，不是失效
- 不要仅因为单根 K 线的最低价跌破 EMA30 就判定趋势失效
- 不要仅因为单根 K 线的最高价突破 EMA30 就判定空头趋势失效
- 必须观察收盘价是否回到趋势方向一侧

### 失效判断
- decision=invalid 需要非常谨慎：必须看到**明确的趋势结构破坏**
- 做多的趋势失效边界：前一段上涨从波段低点到波段高点的 0.618 回撤位，以及该波段低点
  - 只要收盘价没有跌破 0.618 回撤位，也没有跌破前方波段低点，就认为趋势仍在，不能 invalid
  - 仅跌破 EMA30、仅跌破 EMA60、单根放量长阴、单根深 wick，都只能 cooldown/watch，不能 invalid
  - 只有收盘价明确跌破 0.618 回撤位或前方波段低点，才允许 invalid
- 做空的趋势失效边界：前一段下跌从波段高点到波段低点的 0.618 反弹位，以及该波段高点
  - 只要收盘价没有突破 0.618 反弹位，也没有突破前方波段高点，就认为空头趋势仍在，不能 invalid
  - 仅站上 EMA30、仅站上 EMA60、单根放量长阳、单根长上影，都只能 cooldown/watch，不能 invalid
  - 只有收盘价明确突破 0.618 反弹位或前方波段高点，才允许 invalid
- 单根深 wick 后的收盘价仍在 EMA30 附近 = **cooldown**（观察），不是 invalid

### cooldown 状态
- 当价格暂时跌破 EMA30 但未确认趋势破坏时，使用 cooldown
- cooldown 意味着"暂时观望，等待下一根 K 线确认"
- 如果下一根 K 线收回 EMA30 上方，可以恢复 wait/watch

### 不要遗漏买点
- 如果后面的 K 线让你判断"最佳买点已过"，必须把确认买点的那根标为 ready
- 不能全程没有 ready 却事后说买点已过
- 如果已出现支撑收回/放量反包/假跌破收回，且止损可放在回调低点下方，必须评估 ready
- 但如果那根 K 线的风险收益比不足 1.8，仍然不能 ready，只能 watch 并在 risk_notes 说明原因

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

- buy_point=ready: 趋势确认 + 回调/反弹健康 + 出现入场触发信号 + 止损止盈清楚 + 风险收益比>=1.8 + confidence>=70
- 同一段回调/反弹只允许第一次触发 ready，后续续涨/续跌确认不要重复标 ready
- ready 不是趋势延续标签；如果当前 K 线只是继续上涨/下跌、突破前高/前低、或远离 EMA，不要给 ready
- buy_point=watch: 只是接近回调区但没有触发
- buy_point=none: 价格离 EMA 太远或放量加速
- decision=alert: 等价于 buy_point=ready，必须有 entry_price、stop_loss、take_profit
- decision=wait: 继续观察
- decision=cooldown: 暂时不确定（如单根深 wick），等待确认
- decision=invalid: 只有趋势结构明确破坏才使用；做多必须收盘跌破 0.618 回撤位或前波段低点，做空必须收盘突破 0.618 反弹位或前波段高点；trend_state=exhaustion 但结构未破坏时不能 invalid
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

	b.WriteString("\n请按上方方向按时间顺序只分析 observation=true 的K线，并返回 steps 数组。")
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

	b.WriteString("\n请按既定方向分析这些新 K 线，基于之前的上下文继续判断趋势状态和买点/空点。")
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

const minTrendPullbackRiskReward = 1.8

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
			if buyPoint == "ready" && (entryPrice == nil || stopLoss == nil) {
				buyPoint = "watch"
				entryPrice = nil
				stopLoss = nil
				takeProfit = nil
			}
			if buyPoint == "ready" {
				switch {
				case !isStrictTrendPullbackReady(step.TrendState, step.PullbackState, entryPrice, stopLoss, takeProfit):
					buyPoint = "watch"
					entryPrice = nil
					stopLoss = nil
					takeProfit = nil
					riskNotes = appendTrendPullbackGateNote(riskNotes, "未通过系统买点硬校验：需确认趋势、健康回调、止损止盈和风险收益比>=1.8")
				case alertOpen:
					buyPoint = "watch"
					entryPrice = nil
					stopLoss = nil
					takeProfit = nil
					riskNotes = appendTrendPullbackGateNote(riskNotes, "同一段回调已出现买点，避免连续追价重复提醒")
				default:
					alertOpen = true
				}
			} else if shouldResetTrendPullbackAlert(step.PullbackState) {
				alertOpen = false
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
				RiskNotes:     riskNotes,
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
	if decision == "alert" {
		return "wait"
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

func isStrictTrendPullbackReady(trendState, pullbackState string, entryPrice, stopLoss, takeProfit *float64) bool {
	if trendState != "confirmed" {
		return false
	}
	if pullbackState != "healthy" && pullbackState != "completed" {
		return false
	}
	if entryPrice == nil || stopLoss == nil || takeProfit == nil {
		return false
	}
	riskReward, ok := trendPullbackRiskReward(*entryPrice, *stopLoss, *takeProfit)
	return ok && riskReward >= minTrendPullbackRiskReward
}

func trendPullbackRiskReward(entryPrice, stopLoss, takeProfit float64) (float64, bool) {
	if stopLoss < entryPrice && takeProfit > entryPrice {
		risk := entryPrice - stopLoss
		reward := takeProfit - entryPrice
		if risk > 0 && reward > 0 {
			return reward / risk, true
		}
	}
	if stopLoss > entryPrice && takeProfit < entryPrice {
		risk := stopLoss - entryPrice
		reward := entryPrice - takeProfit
		if risk > 0 && reward > 0 {
			return reward / risk, true
		}
	}
	return 0, false
}

func shouldResetTrendPullbackAlert(pullbackState string) bool {
	switch pullbackState {
	case "started", "dangerous":
		return true
	default:
		return false
	}
}

func appendTrendPullbackGateNote(notes []string, note string) []string {
	for _, existing := range notes {
		if existing == note {
			return notes
		}
	}
	return append(notes, note)
}
