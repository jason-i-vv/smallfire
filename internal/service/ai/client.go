package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/smallfire/starfire/internal/config"
)

// ChatMessage 聊天消息
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatRequest OpenAI 兼容请求体
type chatRequest struct {
	Model          string        `json:"model"`
	Messages       []ChatMessage `json:"messages"`
	MaxTokens      int           `json:"max_tokens"`
	Temperature    float64       `json:"temperature"`
	ResponseFormat *struct {
		Type string `json:"type"`
	} `json:"response_format,omitempty"`
}

// chatResponse OpenAI 兼容响应体
type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// AIClient OpenAI 兼容 API 客户端
type AIClient struct {
	baseURL     string
	apiKey      string
	model       string
	maxTokens   int
	temperature float64
	httpClient  *http.Client
}

// NewAIClient 创建 AI 客户端
func NewAIClient(cfg config.AIConfig) *AIClient {
	return &AIClient{
		baseURL:     cfg.BaseURL,
		apiKey:      cfg.APIKey,
		model:       cfg.Model,
		maxTokens:   cfg.MaxTokens,
		temperature: cfg.Temperature,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// ChatCompletion 调用聊天补全 API（强制 JSON 输出）
func (c *AIClient) ChatCompletion(ctx context.Context, messages []ChatMessage) (string, error) {
	return c.ChatCompletionWithMaxTokens(ctx, messages, c.maxTokens)
}

// ChatCompletionWithMaxTokens 调用聊天补全 API（强制 JSON 输出），允许单次覆盖输出长度。
func (c *AIClient) ChatCompletionWithMaxTokens(ctx context.Context, messages []ChatMessage, maxTokens int) (string, error) {
	if maxTokens <= 0 {
		maxTokens = c.maxTokens
	}
	reqBody := chatRequest{
		Model:       c.model,
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: c.temperature,
		ResponseFormat: &struct {
			Type string `json:"type"`
		}{Type: "json_object"},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败: %w", err)
	}

	url := fmt.Sprintf("%s/chat/completions", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonData))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		// 检查是否是 context 取消
		if ctx.Err() == context.Canceled {
			return "", fmt.Errorf("请求被取消: %w", err)
		}
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("请求超时 (context deadline exceeded): %w", err)
		}
		return "", fmt.Errorf("调用 AI API 失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	// 调试日志：记录原始响应
	if len(body) == 0 {
		return "", fmt.Errorf("AI API 返回空响应 (status=%d)", resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("AI API 返回错误 (status=%d): %s", resp.StatusCode, string(body))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("解析响应失败 (raw=%s): %w", string(body), err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("AI API 返回空结果 (raw=%s)", string(body))
	}

	return chatResp.Choices[0].Message.Content, nil
}
