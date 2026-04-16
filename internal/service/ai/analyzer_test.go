package ai

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/smallfire/starfire/internal/config"
	"github.com/smallfire/starfire/internal/models"
)

// TestAnalyzeOpportunity10484 测试 opportunity 10484 的 AI 分析
func TestAnalyzeOpportunity10484(t *testing.T) {
	// 从配置文件或环境变量读取 API Key
	cfg, err := config.Load("../../../config/config.yml")
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}
	apiKey := cfg.AI.APIKey
	if apiKey == "" {
		t.Skip("AI_API_KEY not set in config, skipping test")
	}
	t.Logf("Using API Key: %s...%s", apiKey[:10], apiKey[len(apiKey)-5:])

	opp := &models.TradingOpportunity{
		ID:                   10484,
		SymbolID:             262,
		SymbolCode:           "CHZUSDT",
		Period:               "15m",
		Direction:            "long",
		Score:                65,
		SignalCount:          1,
		ConfluenceDirections:  []string{"resistance_break:long"},
		LastSignalAt:         timePtr(time.Date(2026, 4, 15, 14, 0, 0, 0, time.UTC)),
		FirstSignalAt:        timePtr(time.Date(2026, 4, 15, 14, 0, 0, 0, time.UTC)),
	}

	analyzer := &OpportunityAnalyzer{}
	klineContext := []models.Kline{}
	systemPrompt := analyzer.buildSystemPrompt()
	userPrompt := analyzer.buildUserPrompt(opp, klineContext)

	t.Logf("System Prompt:\n%s", systemPrompt)
	t.Logf("\nUser Prompt:\n%s", userPrompt)

	client := NewAIClient(config.AIConfig{
		BaseURL:     "https://open.bigmodel.cn/api/coding/paas/v4",
		APIKey:      apiKey,
		Model:       "GLM-4-Flash",
		MaxTokens:   500,
		Temperature: 0.3,
	})

	messages := []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	t.Log("Calling AI API...")
	start := time.Now()
	resp, err := client.ChatCompletion(ctx, messages)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("AI API call failed: %v (elapsed: %v)", err, elapsed)
	}

	t.Logf("AI Response (took %v):\n%s", elapsed, resp)
}

// TestDifferentModels 测试不同模型的响应速度
func TestDifferentModels(t *testing.T) {
	apiKey := os.Getenv("AI_API_KEY")
	if apiKey == "" {
		t.Skip("AI_API_KEY not set, skipping test")
	}

	testMessages := []ChatMessage{
		{Role: "user", Content: "简单回复 hello"},
	}

	testModels := []string{
		"GLM-4-Flash",
		"GLM-Z1-AirX",
	}

	for _, model := range testModels {
		t.Run(model, func(t *testing.T) {
			client := NewAIClient(config.AIConfig{
				BaseURL:     "https://open.bigmodel.cn/api/coding/paas/v4",
				APIKey:      apiKey,
				Model:       model,
				MaxTokens:   50,
				Temperature: 0.3,
			})

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			start := time.Now()
			resp, err := client.ChatCompletion(ctx, testMessages)
			elapsed := time.Since(start)

			if err != nil {
				t.Errorf("%s failed: %v", model, err)
				return
			}

			t.Logf("%s: %v - %s", model, elapsed, resp)
		})
	}
}

func timePtr(t time.Time) *time.Time {
	return &t
}
