package ai

import (
	"encoding/json"
	"testing"
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
		{name: "dangerous overrides explicit wait", decision: "wait", buyPoint: "watch", trendState: "weak", pullbackState: "dangerous", want: "invalid"},
		{name: "dangerous derives invalid", decision: "", buyPoint: "none", trendState: "weak", pullbackState: "dangerous", want: "invalid"},
		{name: "exhaustion derives invalid", decision: "", buyPoint: "none", trendState: "exhaustion", pullbackState: "none", want: "invalid"},
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
