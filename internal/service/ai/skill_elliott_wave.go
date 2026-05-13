package ai

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/smallfire/starfire/internal/models"
)

// ElliottWaveSkill 艾略特波浪策略
// 融合 trade-skills/elliott-wave-principle SKILL.md 的核心策略
type ElliottWaveSkill struct{}

func (s *ElliottWaveSkill) Name() string { return "elliott_wave" }

func (s *ElliottWaveSkill) Description() string {
	return "艾略特波浪分析 — 推动浪/修正浪识别 + A股主升低吸买点"
}

func (s *ElliottWaveSkill) SystemPrompt(marketCode string) string {
	base := `你是 smallfire 的艾略特波浪 / 主升低吸买点分析器。只输出一个JSON对象，不要输出Markdown，不要输出解释性正文。

硬性输出要求：
- 第一字符必须是 {，最后字符必须是 }
- 如果无法判断，也必须输出 steps 数组，buy_point 使用 none 或 watch

## 通用艾略特规则

- 先判断大方向，再找推动浪5浪和修正浪3浪，不要强行数浪
- 推动浪规则：2浪不能跌破1浪起点；3浪必须超过1浪终点；3浪不能是1/3/5中最短；非杠杆现货里4浪不能进入1浪价格区间
- 修正浪可为ABC、平台、三角形、组合修正
- Wave 2 常回撤 50%-61.8%，Wave 4 常回撤约38.2%
- 优先寻找 Wave 2 或 Wave 4 修正结束后的低风险买点
- 不要在 Wave 5 末端或抛物线加速时追高
- 买点必须有清楚失效位，风险收益比应至少约1.8

`

	if marketCode == "a_stock" {
		base += `## A股主升低吸规则（优先级高于松散数浪）

### 模型分支选择
- Type B（首选）: 0点 -> 一高 -> 1点 -> 二高 -> 2点，干净主升结构
- Type A: 一高 -> 二高 -> 1点 -> 2点，或一次重置后形成更高2点
- 不要在同一个标注中混用两种模型

### 结构标注规则
- 必须从左到右识别：0点 -> 一高 -> 1点 -> 二高 -> 2点 -> 试盘线/主升段 -> 回踩操盘线买点
- 一高/二高按收盘价判断，二高不能高于一高
- 1点/2点按最低价判断，2点必须高于1点
- 如果二高 >= 一高，重新标注二高为新的一高

### 买点标准
- 买点不是二高突破本身，而是2点确认后出现试盘线/主升段
- 再第一次回踩操盘线（MA20）附近并站稳
- 操盘线优先理解为MA20；若输入没有MA20，用ema_short近似
- 不买：大阴线、连续阴线、不能收回操盘线、跌破2点、结构失败

### BOLL + MACD 时序
- 分析完成后，附加最新收盘的 BOLL 位置和 MACD 状态判断
- 下轨反弹+收回中轨 + MACD水下金叉 = 时序确认
- 上轨反复摩擦 + MACD柱缩小 = 顶部背离警告

### 时序锁定
- 标注必须按时间顺序从左到右
- 一旦确认有效的一高/二高/2点链，不要跳过已有标注选后面的点
- 如果确认的2点被后续跌破，标记旧结构失败，重新建立新结构

`
	}

	base += `## 输出格式

` + "```" + `json
{
  "steps": [
    {
      "kline_index": 0,
      "wave_stage": "wave1|wave2|wave3|wave4|wave5|abc_correction|main_rise_low_buy|unclear|invalidated",
      "pattern_type": "impulse|correction|type_a|type_b|tracking|failed|unclear",
      "setup_status": "confirmed|tracking|warning|invalidated",
      "buy_point": "none|watch|ready",
      "decision": "wait|alert|invalid|cooldown",
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
` + "```" + `

## 判定规则
- buy_point=ready: 结构 confirmed、出现明确低吸/修正结束/突破回踩触发、止损位清楚、confidence>=70
- buy_point=watch: tracking 或结构未完成
- buy_point=none: Wave 5 末端、抛物线加速、跌破2点、放量破操盘线
- decision=alert 等价于 buy_point=ready，必须有 entry_price、stop_loss、target_price
- decision=invalid: 结构明确失败（如确认的2点被跌破）
- decision=cooldown: 暂时不确定，等待下一根确认
- steps 必须只包含 observation=true 的K线，kline_index 使用输入中的 index`

	return base
}

func (s *ElliottWaveSkill) BuildFirstMessage(klines []models.Kline, observationStart int) string {
	var b strings.Builder
	b.WriteString("首次分析。K线按时间正序排列。observation=false 是结构背景上下文；observation=true 是需要逐根回放判断的观察K线。\n")
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

func (s *ElliottWaveSkill) BuildIncrementalMessage(klines []models.Kline) string {
	var b strings.Builder
	b.WriteString("新 K 线到达，请基于之前的波浪/主升结构分析继续判断：\n")
	b.WriteString("字段: index time open high low close volume ema_short ema_medium ema_long\n\n")

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

	b.WriteString("\n请分析这些新 K 线，基于之前的结构继续判断波浪阶段和买点。")
	return b.String()
}

// skillWaveAIStep AI 返回的波浪单步结果
type skillWaveAIStep struct {
	KlineIndex        int      `json:"kline_index"`
	WaveStage         string   `json:"wave_stage"`
	PatternType       string   `json:"pattern_type"`
	SetupStatus       string   `json:"setup_status"`
	BuyPoint          string   `json:"buy_point"`
	Decision          string   `json:"decision"`
	EntryPrice        *float64 `json:"entry_price"`
	StopLoss          *float64 `json:"stop_loss"`
	TargetPrice       *float64 `json:"target_price"`
	InvalidationLevel *float64 `json:"invalidation_level"`
	Confidence        int      `json:"confidence"`
	Reasoning         string   `json:"reasoning"`
	WaveCount         string   `json:"wave_count"`
	RiskNotes         []string `json:"risk_notes"`
}

type skillWaveAIResult struct {
	Steps []skillWaveAIStep `json:"steps"`
}

func (s *ElliottWaveSkill) ParseResponse(raw string) ([]AnalysisStep, error) {
	candidates := extractJSONCandidates(raw)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("未找到 JSON 对象")
	}

	var lastErr error
	for i := len(candidates) - 1; i >= 0; i-- {
		var result skillWaveAIResult
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
			targetPrice := normalizeOptionalPrice(step.TargetPrice)

			if buyPoint != "ready" {
				entryPrice = nil
				stopLoss = nil
				targetPrice = nil
			}
			if buyPoint == "ready" && (entryPrice == nil || stopLoss == nil) {
				buyPoint = "watch"
				entryPrice = nil
				stopLoss = nil
				targetPrice = nil
			}

			decision := normalizeWaveDecision(step.Decision, buyPoint, step.SetupStatus)

			extra := map[string]interface{}{
				"wave_stage":   step.WaveStage,
				"pattern_type": step.PatternType,
				"setup_status": step.SetupStatus,
				"wave_count":   step.WaveCount,
			}
			if targetPrice != nil {
				extra["target_price"] = *targetPrice
			}

			steps = append(steps, AnalysisStep{
				BuyPoint:   buyPoint,
				Decision:   decision,
				EntryPrice: entryPrice,
				StopLoss:   stopLoss,
				Confidence: normalizeConfidence(step.Confidence),
				Reasoning:  step.Reasoning,
				RiskNotes:  step.RiskNotes,
				Extra:      extra,
			})
		}
		return steps, nil
	}

	return nil, lastErr
}

func normalizeWaveDecision(decision, buyPoint, setupStatus string) string {
	if buyPoint == "ready" {
		return "alert"
	}
	if setupStatus == "invalidated" {
		return "invalid"
	}
	switch decision {
	case "alert", "wait", "invalid", "cooldown":
		return decision
	}
	return "wait"
}
