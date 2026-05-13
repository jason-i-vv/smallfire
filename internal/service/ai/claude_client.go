package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/smallfire/starfire/internal/config"
	"go.uber.org/zap"
)

// ClaudeMessage 会话消息
type ClaudeMessage struct {
	Role    string `json:"role"`    // "user" or "assistant"
	Content string `json:"content"`
}

// ClaudeConversation 会话（持久化为 JSON 文件）
type ClaudeConversation struct {
	SystemPrompt string          `json:"system_prompt"`
	Messages     []ClaudeMessage `json:"messages"`
	TargetID     int             `json:"target_id"`
	SkillName    string          `json:"skill_name"`
	CreatedAt    int64           `json:"created_at"`
	UpdatedAt    int64           `json:"updated_at"`
}

// ClaudeClient Claude API 客户端
type ClaudeClient struct {
	client    *anthropic.Client
	model     string
	maxTokens int
	convDir   string // 会话文件目录
	logger    *zap.Logger
}

// NewClaudeClient 创建 Claude 客户端
func NewClaudeClient(cfg config.ClaudeConfig, logDir string, logger *zap.Logger) *ClaudeClient {
	if !cfg.Enabled {
		return nil
	}

	opts := []option.RequestOption{option.WithAPIKey(cfg.APIKey)}
	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	}
	client := anthropic.NewClient(opts...)

	convDir := cfg.ConversationDir
	if !filepath.IsAbs(convDir) {
		convDir = filepath.Join(logDir, convDir)
	}

	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = 4096
	}
	if cfg.Model == "" {
		cfg.Model = "claude-sonnet-4-6"
	}

	return &ClaudeClient{
		client:    &client,
		model:     cfg.Model,
		maxTokens: cfg.MaxTokens,
		convDir:   convDir,
		logger:    logger,
	}
}

// Chat 调用 Claude Messages API（使用 streaming 模式）
func (c *ClaudeClient) Chat(ctx context.Context, systemPrompt string, messages []ClaudeMessage) (string, error) {
	if c == nil || c.client == nil {
		return "", fmt.Errorf("Claude 客户端未初始化")
	}

	// 转换消息格式
	anthropicMessages := make([]anthropic.MessageParam, 0, len(messages))
	for _, msg := range messages {
		switch msg.Role {
		case "user":
			anthropicMessages = append(anthropicMessages, anthropic.NewUserMessage(
				anthropic.NewTextBlock(msg.Content),
			))
		case "assistant":
			anthropicMessages = append(anthropicMessages, anthropic.NewAssistantMessage(
				anthropic.NewTextBlock(msg.Content),
			))
		}
	}

	params := anthropic.MessageNewParams{
		Model:     c.model,
		MaxTokens: int64(c.maxTokens),
		System:    []anthropic.TextBlockParam{{Text: systemPrompt}},
		Messages:  anthropicMessages,
	}

	// 使用 streaming 模式，避免 "streaming is required" 错误
	stream := c.client.Messages.NewStreaming(ctx, params)
	var text string
	for stream.Next() {
		event := stream.Current()
		switch variant := event.AsAny().(type) {
		case anthropic.ContentBlockDeltaEvent:
			if variant.Delta.Type == "text_delta" {
				text += variant.Delta.Text
			}
		}
	}
	if err := stream.Err(); err != nil {
		return "", fmt.Errorf("Claude API 调用失败: %w", err)
	}

	if text == "" {
		return "", fmt.Errorf("Claude API 返回空响应")
	}

	return text, nil
}

// LoadConversation 从文件加载会话
func (c *ClaudeClient) LoadConversation(targetID int) (*ClaudeConversation, error) {
	if c == nil {
		return nil, fmt.Errorf("Claude 客户端未初始化")
	}

	path := c.conversationPath(targetID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // 首次分析，无会话文件
		}
		return nil, fmt.Errorf("读取会话文件失败: %w", err)
	}

	var conv ClaudeConversation
	if err := json.Unmarshal(data, &conv); err != nil {
		return nil, fmt.Errorf("解析会话文件失败: %w", err)
	}

	return &conv, nil
}

// SaveConversation 保存会话到文件
func (c *ClaudeClient) SaveConversation(conv *ClaudeConversation) error {
	if c == nil {
		return fmt.Errorf("Claude 客户端未初始化")
	}

	if err := os.MkdirAll(c.convDir, 0755); err != nil {
		return fmt.Errorf("创建会话目录失败: %w", err)
	}

	conv.UpdatedAt = time.Now().UnixMilli()
	data, err := json.MarshalIndent(conv, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化会话失败: %w", err)
	}

	path := c.conversationPath(conv.TargetID)
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("写入会话文件失败: %w", err)
	}

	return os.Rename(tmpPath, path)
}

// ResetConversation 重置会话（删除文件）
func (c *ClaudeClient) ResetConversation(targetID int) error {
	if c == nil {
		return fmt.Errorf("Claude 客户端未初始化")
	}

	path := c.conversationPath(targetID)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除会话文件失败: %w", err)
	}
	return nil
}

// CompressConversation 压缩会话历史（保留最近 N 轮，更早的摘要为一条 user 消息）
func (c *ClaudeClient) CompressConversation(conv *ClaudeConversation, maxKeepPairs int) *ClaudeConversation {
	if len(conv.Messages) <= maxKeepPairs*2 {
		return conv
	}

	// 保留最近 maxKeepPairs 轮（每轮 = 1 user + 1 assistant = 2 条）
	cutIndex := len(conv.Messages) - maxKeepPairs*2
	oldMessages := conv.Messages[:cutIndex]
	newMessages := conv.Messages[cutIndex:]

	// 将旧消息摘要为一条
	summary := "以下是之前的分析摘要（已压缩）：\n"
	for i := 0; i < len(oldMessages); i += 2 {
		if i+1 < len(oldMessages) {
			summary += fmt.Sprintf("- 用户输入 K 线数据\n- AI 回复: %s\n", truncateString(oldMessages[i+1].Content, 200))
		}
	}

	result := &ClaudeConversation{
		SystemPrompt: conv.SystemPrompt,
		TargetID:     conv.TargetID,
		SkillName:    conv.SkillName,
		CreatedAt:    conv.CreatedAt,
		UpdatedAt:    time.Now().UnixMilli(),
		Messages: append([]ClaudeMessage{
			{Role: "user", Content: summary},
		}, newMessages...),
	}

	return result
}

func (c *ClaudeClient) conversationPath(targetID int) string {
	return filepath.Join(c.convDir, fmt.Sprintf("%d.json", targetID))
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
