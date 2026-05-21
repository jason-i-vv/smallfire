package ai

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/smallfire/starfire/internal/models"
)

func TestExtractJSONFromThinkResponse(t *testing.T) {
	raw := `<think>
这里是模型推理内容，可能包含大量中文分析。
</think>

{"trend_state":"exhaustion","pullback_state":"none","buy_point":"none","confidence":35}`

	var parsed map[string]any
	if err := json.Unmarshal([]byte(extractJSON(raw)), &parsed); err != nil {
		t.Fatalf("extractJSON returned invalid JSON: %v", err)
	}

	if parsed["trend_state"] != "exhaustion" {
		t.Fatalf("unexpected trend_state: %v", parsed["trend_state"])
	}
}

func TestParseTrendPullbackBatchResponse(t *testing.T) {
	raw := `<think>批量回放观察K线</think>
{
  "steps": [
    {
      "kline_index": 42,
      "trend_state": "confirmed",
      "pullback_state": "healthy",
      "buy_point": "ready",
      "decision": "alert",
      "entry_price": 1.23,
      "stop_loss": 1.18,
      "take_profit": 1.33,
      "invalidation_level": 1.17,
      "confidence": 76,
      "missed": false,
      "missed_kline_index": 0,
      "reasoning": "回踩EMA30后反包",
      "risk_notes": ["止损需严格执行"]
    }
  ]
}`

	parsed, err := parseTrendPullbackBatchResponse(raw)
	if err != nil {
		t.Fatalf("parseTrendPullbackBatchResponse failed: %v", err)
	}
	if len(parsed.Steps) != 1 {
		t.Fatalf("unexpected step count: %d", len(parsed.Steps))
	}
	if parsed.Steps[0].KlineIndex != 42 {
		t.Fatalf("unexpected kline_index: %d", parsed.Steps[0].KlineIndex)
	}
	if parsed.Steps[0].Decision != "alert" {
		t.Fatalf("unexpected decision: %s", parsed.Steps[0].Decision)
	}
}

func TestParseTrendPullbackBatchResponseSkipsThinkFragments(t *testing.T) {
	raw := `<think>
模型先写了半截步骤，里面也有合法对象，但不是最终结果：
{"kline_index": 115, "trend_state": "weak", "pullback_state": "completed", "buy_point": "watch"}
继续推理后才给最终 JSON。
</think>

{"steps":[
  {
    "kline_index": 119,
    "trend_state": "confirmed",
    "pullback_state": "completed",
    "buy_point": "ready",
    "entry_price": 81200,
    "stop_loss": 80900,
    "take_profit": 81787,
    "invalidation_level": 80638,
    "confidence": 85,
    "reasoning": "阳线收回EMA30上方",
    "risk_notes": ["跌破80638前低结构破坏离场"]
  }
]}`

	parsed, err := parseTrendPullbackBatchResponse(raw)
	if err != nil {
		t.Fatalf("parseTrendPullbackBatchResponse failed: %v", err)
	}
	if len(parsed.Steps) != 1 {
		t.Fatalf("unexpected step count: %d", len(parsed.Steps))
	}
	if parsed.Steps[0].KlineIndex != 119 {
		t.Fatalf("unexpected kline_index: %d", parsed.Steps[0].KlineIndex)
	}
	if parsed.Steps[0].BuyPoint != "ready" {
		t.Fatalf("unexpected buy_point: %s", parsed.Steps[0].BuyPoint)
	}
}

func TestParseTrendPullbackBatchResponseWithoutJSON(t *testing.T) {
	raw := `<think>只有思考过程，没有结构化JSON</think>`

	if _, err := parseTrendPullbackBatchResponse(raw); err == nil {
		t.Fatal("expected parseTrendPullbackBatchResponse to fail when JSON is missing")
	}
}

func TestNormalizeTrendPullbackConfidence(t *testing.T) {
	tests := []struct {
		name     string
		buyPoint string
		input    int
		want     int
	}{
		{name: "none is always zero", buyPoint: "none", input: 80, want: 0},
		{name: "watch zero becomes candidate confidence", buyPoint: "watch", input: 0, want: 50},
		{name: "watch upper bound", buyPoint: "watch", input: 95, want: 69},
		{name: "ready lower bound", buyPoint: "ready", input: 0, want: 70},
		{name: "ready keeps high confidence", buyPoint: "ready", input: 82, want: 82},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeTrendPullbackConfidence(tt.buyPoint, tt.input)
			if got != tt.want {
				t.Fatalf("normalizeTrendPullbackConfidence() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestNormalizeTrendPullbackDecision(t *testing.T) {
	tests := []struct {
		name          string
		decision      string
		buyPoint      string
		trendState    string
		pullbackState string
		want          string
	}{
		{name: "ready always alerts", decision: "wait", buyPoint: "ready", trendState: "confirmed", pullbackState: "healthy", want: "alert"},
		{name: "keeps explicit wait", decision: "wait", buyPoint: "watch", trendState: "confirmed", pullbackState: "healthy", want: "wait"},
		{name: "dangerous keeps explicit wait", decision: "wait", buyPoint: "watch", trendState: "weak", pullbackState: "dangerous", want: "wait"},
		{name: "dangerous without explicit invalid derives wait", decision: "", buyPoint: "none", trendState: "weak", pullbackState: "dangerous", want: "wait"},
		{name: "exhaustion without structure break derives wait", decision: "", buyPoint: "none", trendState: "exhaustion", pullbackState: "none", want: "wait"},
		{name: "explicit invalid is preserved", decision: "invalid", buyPoint: "none", trendState: "exhaustion", pullbackState: "dangerous", want: "invalid"},
		{name: "default derives wait", decision: "", buyPoint: "watch", trendState: "confirmed", pullbackState: "started", want: "wait"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeTrendPullbackDecision(tt.decision, tt.buyPoint, tt.trendState, tt.pullbackState)
			if got != tt.want {
				t.Fatalf("normalizeTrendPullbackDecision() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestSkillDecisionDoesNotPromoteAlertWithoutReadyBuyPoint(t *testing.T) {
	trend := &TrendPullbackSkill{}
	if got := trend.normalizeDecision("alert", "none", "confirmed", "healthy"); got != "wait" {
		t.Fatalf("trend normalizeDecision() = %s, want wait", got)
	}
	if got := trend.normalizeDecision("wait", "none", "exhaustion", "completed"); got != "wait" {
		t.Fatalf("trend normalizeDecision() = %s, want wait for exhaustion without structure break", got)
	}
	if got := trend.normalizeDecision("invalid", "none", "exhaustion", "completed"); got != "invalid" {
		t.Fatalf("trend normalizeDecision() = %s, want explicit invalid to be preserved", got)
	}
	if got := normalizeWaveDecision("alert", "none", "tracking"); got != "wait" {
		t.Fatalf("normalizeWaveDecision() = %s, want wait", got)
	}
	if got := normalizeWaveDecision("alert", "ready", "confirmed"); got != "alert" {
		t.Fatalf("normalizeWaveDecision() = %s, want alert for ready buy point", got)
	}
}

func TestTrendPullbackExhaustionDoesNotInvalidateWithoutStructureBreak(t *testing.T) {
	raw := `{"steps":[
		{
			"kline_index": 114,
			"trend_state": "exhaustion",
			"pullback_state": "completed",
			"buy_point": "none",
			"decision": "wait",
			"entry_price": 0,
			"stop_loss": 0,
			"take_profit": 0,
			"confidence": 0,
			"reasoning": "单根暴涨抛物线加速，过热不追"
		}
	]}`

	steps, err := (&TrendPullbackSkill{}).ParseResponse(raw)
	if err != nil {
		t.Fatalf("ParseResponse failed: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("unexpected step count: %d", len(steps))
	}
	if steps[0].Decision != "wait" {
		t.Fatalf("Decision = %s, want wait", steps[0].Decision)
	}
	if steps[0].BuyPoint != "none" {
		t.Fatalf("BuyPoint = %s, want none", steps[0].BuyPoint)
	}
}

func TestTrendPullbackInvalidGuardKeepsLongTrendAboveStructure(t *testing.T) {
	base := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	klines := []models.Kline{
		{OpenTime: base, HighPrice: 0.0075, LowPrice: 0.00736, ClosePrice: 0.0074},
		{OpenTime: base.Add(time.Hour), HighPrice: 0.0082, LowPrice: 0.0078, ClosePrice: 0.0081},
		{OpenTime: base.Add(2 * time.Hour), HighPrice: 0.0091, LowPrice: 0.008, ClosePrice: 0.009},
		{OpenTime: base.Add(3 * time.Hour), HighPrice: 0.0098, LowPrice: 0.0089, ClosePrice: 0.0097},
		{OpenTime: base.Add(4 * time.Hour), HighPrice: 0.01043, LowPrice: 0.0095, ClosePrice: 0.0102},
		{OpenTime: base.Add(5 * time.Hour), HighPrice: 0.0099, LowPrice: 0.009165, ClosePrice: 0.009442},
	}
	steps := []AnalysisStep{{
		KlineTime:  klines[5].OpenTime.UnixMilli(),
		ClosePrice: klines[5].ClosePrice,
		Decision:   "invalid",
		RiskNotes:  []string{"跌破EMA30"},
	}}

	guarded := guardTrendPullbackInvalidSteps(models.DirectionLong, steps, klines)
	if guarded[0].Decision != "cooldown" {
		t.Fatalf("Decision = %s, want cooldown", guarded[0].Decision)
	}
}

func TestTrendPullbackInvalidGuardKeepsShortTrendBelowStructure(t *testing.T) {
	base := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	klines := []models.Kline{
		{OpenTime: base, HighPrice: 10, LowPrice: 9.8, ClosePrice: 9.9},
		{OpenTime: base.Add(time.Hour), HighPrice: 9.7, LowPrice: 9.1, ClosePrice: 9.2},
		{OpenTime: base.Add(2 * time.Hour), HighPrice: 9.3, LowPrice: 8.4, ClosePrice: 8.5},
		{OpenTime: base.Add(3 * time.Hour), HighPrice: 8.7, LowPrice: 8, ClosePrice: 8.1},
		{OpenTime: base.Add(4 * time.Hour), HighPrice: 8.9, LowPrice: 8.2, ClosePrice: 8.7},
	}
	steps := []AnalysisStep{{
		KlineTime:  klines[4].OpenTime.UnixMilli(),
		ClosePrice: klines[4].ClosePrice,
		Decision:   "invalid",
		RiskNotes:  []string{"站上EMA30"},
	}}

	guarded := guardTrendPullbackInvalidSteps(models.DirectionShort, steps, klines)
	if guarded[0].Decision != "cooldown" {
		t.Fatalf("Decision = %s, want cooldown", guarded[0].Decision)
	}
}

func TestTrendPullbackReadyRequiresValidRiskReward(t *testing.T) {
	raw := `{"steps":[
		{
			"kline_index": 1,
			"trend_state": "confirmed",
			"pullback_state": "completed",
			"buy_point": "ready",
			"decision": "alert",
			"entry_price": 0.09112,
			"stop_loss": 0.0835,
			"take_profit": 0.095,
			"confidence": 85,
			"reasoning": "续涨接近前高"
		}
	]}`

	steps, err := (&TrendPullbackSkill{}).ParseResponse(raw)
	if err != nil {
		t.Fatalf("ParseResponse failed: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("unexpected step count: %d", len(steps))
	}
	if steps[0].BuyPoint != "watch" {
		t.Fatalf("BuyPoint = %s, want watch", steps[0].BuyPoint)
	}
	if steps[0].Decision != "wait" {
		t.Fatalf("Decision = %s, want wait", steps[0].Decision)
	}
	if steps[0].EntryPrice != nil || steps[0].StopLoss != nil || steps[0].TakeProfit != nil {
		t.Fatalf("expected downgraded step to clear prices: %+v", steps[0])
	}
	if steps[0].Confidence > 69 {
		t.Fatalf("Confidence = %d, want <= 69", steps[0].Confidence)
	}
}

func TestTrendPullbackReadyKeepsValidRiskRewardAndDedupesSamePullback(t *testing.T) {
	raw := `{"steps":[
		{
			"kline_index": 1,
			"trend_state": "confirmed",
			"pullback_state": "completed",
			"buy_point": "ready",
			"decision": "alert",
			"entry_price": 10,
			"stop_loss": 9,
			"take_profit": 12,
			"confidence": 75,
			"reasoning": "回踩EMA30后反包"
		},
		{
			"kline_index": 2,
			"trend_state": "confirmed",
			"pullback_state": "completed",
			"buy_point": "ready",
			"decision": "alert",
			"entry_price": 10.2,
			"stop_loss": 9.2,
			"take_profit": 12.3,
			"confidence": 80,
			"reasoning": "继续上涨"
		}
	]}`

	steps, err := (&TrendPullbackSkill{}).ParseResponse(raw)
	if err != nil {
		t.Fatalf("ParseResponse failed: %v", err)
	}
	if len(steps) != 2 {
		t.Fatalf("unexpected step count: %d", len(steps))
	}
	if steps[0].BuyPoint != "ready" || steps[0].Decision != "alert" {
		t.Fatalf("first step = %s/%s, want ready/alert", steps[0].BuyPoint, steps[0].Decision)
	}
	if steps[1].BuyPoint != "watch" || steps[1].Decision != "wait" {
		t.Fatalf("second step = %s/%s, want watch/wait", steps[1].BuyPoint, steps[1].Decision)
	}
}

func TestInvalidStepTerminatesTrackingImmediately(t *testing.T) {
	steps := []AnalysisStep{
		{Decision: "wait"},
		{Decision: "invalid"},
		{Decision: "wait"},
	}
	if got := firstInvalidStep(steps); got == nil || got.Decision != "invalid" {
		t.Fatalf("firstInvalidStep() = %v, want invalid step", got)
	}

	raw := json.RawMessage(`[{"decision":"wait"},{"decision":"invalid"}]`)
	if !hasInvalidStepJSON(raw) {
		t.Fatal("hasInvalidStepJSON() = false, want true")
	}
}

func TestNormalizeMissedKlineIndex(t *testing.T) {
	valid := 42
	if got := normalizeMissedKlineIndex(true, &valid, 40, 50); got == nil || *got != valid {
		t.Fatalf("expected valid missed index, got %v", got)
	}
	if got := normalizeMissedKlineIndex(false, &valid, 40, 50); got != nil {
		t.Fatalf("expected nil when missed is false, got %v", got)
	}
	invalid := 39
	if got := normalizeMissedKlineIndex(true, &invalid, 40, 50); got != nil {
		t.Fatalf("expected nil for out-of-window index, got %v", got)
	}
}

func TestParseElliottWaveBatchResponse(t *testing.T) {
	raw := `<think>按观察K线回放波浪结构</think>
{
  "steps": [
    {
      "kline_index": 88,
      "wave_stage": "main_rise_low_buy",
      "pattern_type": "type_b",
      "setup_status": "confirmed",
      "buy_point": "ready",
      "entry_price": 12.3,
      "stop_loss": 11.6,
      "target_price": 14.5,
      "invalidation_level": 11.5,
      "confidence": 78,
      "reasoning": "2点后试盘线回踩操盘线站稳",
      "wave_count": "0点-一高-1低-二高-2低-试盘线",
      "risk_notes": ["跌破2点失效"]
    }
  ]
}`

	parsed, err := parseElliottWaveBatchResponse(raw)
	if err != nil {
		t.Fatalf("parseElliottWaveBatchResponse failed: %v", err)
	}
	if len(parsed.Steps) != 1 {
		t.Fatalf("unexpected step count: %d", len(parsed.Steps))
	}
	if parsed.Steps[0].PatternType != "type_b" {
		t.Fatalf("unexpected pattern_type: %s", parsed.Steps[0].PatternType)
	}
}

func TestParseElliottWaveBatchResponseSkipsThinkFragments(t *testing.T) {
	raw := `<think>
{"kline_index": 60, "wave_stage": "wave4", "pattern_type": "correction", "buy_point": "watch"}
</think>

{"steps":[
  {
    "kline_index": 91,
    "wave_stage": "main_rise_low_buy",
    "pattern_type": "type_a",
    "setup_status": "confirmed",
    "buy_point": "ready",
    "entry_price": 21.3,
    "stop_loss": 20.1,
    "target_price": 25.5,
    "invalidation_level": 19.9,
    "confidence": 81,
    "reasoning": "2点后首次回踩操盘线站稳",
    "wave_count": "0点-一高-二高-1低-2低-试盘线",
    "risk_notes": ["跌破2点失效"]
  }
]}`

	parsed, err := parseElliottWaveBatchResponse(raw)
	if err != nil {
		t.Fatalf("parseElliottWaveBatchResponse failed: %v", err)
	}
	if len(parsed.Steps) != 1 {
		t.Fatalf("unexpected step count: %d", len(parsed.Steps))
	}
	if parsed.Steps[0].KlineIndex != 91 {
		t.Fatalf("unexpected kline_index: %d", parsed.Steps[0].KlineIndex)
	}
	if parsed.Steps[0].PatternType != "type_a" {
		t.Fatalf("unexpected pattern_type: %s", parsed.Steps[0].PatternType)
	}
}

func TestParseElliottWaveBatchResponseWithoutJSON(t *testing.T) {
	raw := `<think>只有思考过程，没有结构化JSON</think>`

	if _, err := parseElliottWaveBatchResponse(raw); err == nil {
		t.Fatal("expected parseElliottWaveBatchResponse to fail when JSON is missing")
	}
}
