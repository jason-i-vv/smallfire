package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/smallfire/starfire/internal/config"
	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/internal/repository"
	"go.uber.org/zap"
)

// AIKeyLevelAnalyzer AI 关键价位识别器
type AIKeyLevelAnalyzer struct {
	client     *AIClient
	klineRepo  repository.KlineRepo
	levelV2Repo repository.KeyLevelV2Repo
	klineRepo2 interface {
		GetAllTrackedSymbols() ([]*repository.TrackedSymbol, error)
	}
	config   config.AIKeyLevelConfig
	cooldown *CooldownTracker
	stopCh   chan struct{}
	wg       sync.WaitGroup
	logger   *zap.Logger
}

// NewAIKeyLevelAnalyzer 创建 AI 关键价位识别器
func NewAIKeyLevelAnalyzer(
	client *AIClient,
	klineRepo repository.KlineRepo,
	levelV2Repo repository.KeyLevelV2Repo,
	symbolRepo interface {
		GetAllTrackedSymbols() ([]*repository.TrackedSymbol, error)
	},
	cfg config.AIKeyLevelConfig,
	cooldown *CooldownTracker,
	logger *zap.Logger,
) *AIKeyLevelAnalyzer {
	return &AIKeyLevelAnalyzer{
		client:      client,
		klineRepo:   klineRepo,
		levelV2Repo: levelV2Repo,
		klineRepo2:  symbolRepo,
		config:      cfg,
		cooldown:    cooldown,
		stopCh:      make(chan struct{}),
		logger:      logger,
	}
}

// Run 启动定时分析循环
func (a *AIKeyLevelAnalyzer) Run() {
	interval := time.Duration(a.config.IntervalMinutes) * time.Minute
	if interval < 10*time.Minute {
		interval = 10 * time.Minute
	}

	a.logger.Info("AI关键价位识别器启动",
		zap.Duration("interval", interval),
		zap.Int("max_daily_calls", a.config.MaxDailyCalls))

	// 首次启动延迟 30 秒，等待系统就绪
	select {
	case <-time.After(30 * time.Second):
	case <-a.stopCh:
		return
	}

	// 首次立即执行
	a.runOnce()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			a.runOnce()
		case <-a.stopCh:
			a.logger.Info("AI关键价位识别器停止")
			return
		}
	}
}

// Stop 停止分析循环
func (a *AIKeyLevelAnalyzer) Stop() {
	close(a.stopCh)
	a.wg.Wait()
}

// runOnce 执行一轮全量分析（只分析1h周期，其他周期共用）
func (a *AIKeyLevelAnalyzer) runOnce() {
	symbols, err := a.klineRepo2.GetAllTrackedSymbols()
	if err != nil {
		a.logger.Error("获取跟踪标的失败", zap.Error(err))
		return
	}

	a.logger.Info("开始AI关键价位分析", zap.Int("symbol_count", len(symbols)))

	// 限流间隔
	reqInterval := time.Duration(a.config.RequestInterval) * time.Second
	if reqInterval < time.Second {
		reqInterval = 3 * time.Second
	}

	successCount := 0
	for _, symbol := range symbols {
		// 所有市场统一使用 1h 周期
		period := "1h"

		if ok, reason := a.cooldown.CanAnalyze(symbol.ID); !ok {
			a.logger.Debug("AI价位分析跳过(冷却中)",
				zap.String("symbol", symbol.Code),
				zap.String("reason", reason))
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		err := a.AnalyzeKeyLevels(ctx, symbol.ID, symbol.Code, period)
		cancel()

		if err != nil {
			a.logger.Error("AI关键价位分析失败",
				zap.String("symbol", symbol.Code),
				zap.Error(err))
			// 限流错误时加长等待
			if strings.Contains(err.Error(), "429") {
				a.logger.Warn("触发限流，等待60秒后继续")
				select {
				case <-time.After(60 * time.Second):
				case <-a.stopCh:
					return
				}
			}
			continue
		}

		a.cooldown.Record(symbol.ID)
		successCount++

		// 限流：每次调用后等待
		select {
		case <-time.After(reqInterval):
		case <-a.stopCh:
			return
		}
	}

	a.logger.Info("AI关键价位分析完成", zap.Int("success", successCount), zap.Int("total", len(symbols)))
}

// AnalyzeKeyLevels 对单个标的+周期调用 AI 识别关键价位
func (a *AIKeyLevelAnalyzer) AnalyzeKeyLevels(ctx context.Context, symbolID int, symbolCode, period string) error {
	// 1. 获取K线数据
	klineCount := a.config.KlineCount
	if klineCount <= 0 {
		klineCount = 200
	}
	klines, err := a.klineRepo.GetLatestN(symbolID, period, klineCount)
	if err != nil {
		return fmt.Errorf("获取K线数据失败: %w", err)
	}
	if len(klines) < 30 {
		// K线不足，静默跳过
		return nil
	}

	// 2. 构建 prompt
	systemPrompt := a.buildSystemPrompt()
	userPrompt := a.buildUserPrompt(symbolCode, period, klines)

	// 3. 调用 AI
	messages := []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	response, err := a.client.ChatCompletion(ctx, messages)
	if err != nil {
		return fmt.Errorf("AI API调用失败: %w", err)
	}

	// 4. 解析结果（V2格式）
	result, err := parseAIKeyLevelResponseV2(response)
	if err != nil {
		return fmt.Errorf("解析AI响应失败: %w", err)
	}

	// 5. Upsert 到 V2 表
	if err := a.levelV2Repo.Upsert(symbolID, period, result.Resistances, result.Supports); err != nil {
		return fmt.Errorf("保存关键价位失败: %w", err)
	}

	a.logger.Info("AI关键价位更新",
		zap.String("symbol", symbolCode),
		zap.String("period", period),
		zap.Int("resistance_count", len(result.Resistances)),
		zap.Int("support_count", len(result.Supports)))

	return nil
}

// buildSystemPrompt 构建系统提示词
func (a *AIKeyLevelAnalyzer) buildSystemPrompt() string {
	return `你是专业的技术分析师。根据提供的K线数据，识别关键支撑位和阻力位。
只输出一个JSON对象，不要输出其他内容。不要用markdown代码块包裹。

输出格式：
{"resistances":[{"price":85200.00,"strength":"strong","reason":"3次测试未能有效突破+整数关口"}],"supports":[{"price":84000.00,"strength":"mid","reason":"EMA60均线支撑+2次测试"}]}

规则：
1. 只识别真正的关键价位，不要报告每个小波动。
2. strength 取值：strong(强,3+次触及或整数关口或历史高低点)、mid(中,2-3次触及)、weak(弱,1-2次触及)
3. 每种强度最多1个价位（每个方向最多3个：strong/mid/weak各1个），价格精确到小数点后2位
4. 优先识别距离当前价格较近的关键价位
5. 注意识别整数关口（如85000、86000等）作为心理支撑/阻力
6. reason 要简短描述识别理由（10字以内）`
}

// buildUserPrompt 构建用户消息
func (a *AIKeyLevelAnalyzer) buildUserPrompt(symbolCode, period string, klines []models.Kline) string {
	var sb strings.Builder

	// 基本信息
	lastKline := klines[len(klines)-1]
	sb.WriteString(fmt.Sprintf("标的: %s\n", symbolCode))
	sb.WriteString(fmt.Sprintf("周期: %s\n", period))
	sb.WriteString(fmt.Sprintf("当前价: %.2f\n\n", lastKline.ClosePrice))

	// K线数据（最近20根，节省 token）
	sb.WriteString("近期K线(最近20根):\n")
	start := 0
	if len(klines) > 20 {
		start = len(klines) - 20
	}
	recentKlines := klines[start:]
	for _, k := range recentKlines {
		t := k.OpenTime.Format("01-02 15:04")
		change := ""
		if k.OpenPrice > 0 {
			change = fmt.Sprintf(" (%+.2f%%)", (k.ClosePrice-k.OpenPrice)/k.OpenPrice*100)
		}
		sb.WriteString(fmt.Sprintf("  %s O:%.2f H:%.2f L:%.2f C:%.2f Vol:%.0f%s\n",
			t, k.OpenPrice, k.HighPrice, k.LowPrice, k.ClosePrice, k.Volume, change))
	}

	// EMA 趋势
	if lastKline.EMAShort != nil && lastKline.EMAMedium != nil && lastKline.EMALong != nil {
		sb.WriteString(fmt.Sprintf("\n当前均线: EMA30=%.2f EMA60=%.2f EMA90=%.2f\n",
			*lastKline.EMAShort, *lastKline.EMAMedium, *lastKline.EMALong))
	}

	// 成交量均值
	if len(klines) >= 20 {
		var sumVol float64
		for i := len(klines) - 20; i < len(klines); i++ {
			sumVol += klines[i].Volume
		}
		avgVol := sumVol / 20.0
		sb.WriteString(fmt.Sprintf("20周期均量: %.0f\n", avgVol))
	}

	// 区间统计
	var high, low float64
	for i, k := range klines {
		if i == 0 || k.HighPrice > high {
			high = k.HighPrice
		}
		if i == 0 || k.LowPrice < low {
			low = k.LowPrice
		}
	}
	sb.WriteString(fmt.Sprintf("近期区间: 最高=%.2f 最低=%.2f 振幅=%.2f%%\n",
		high, low, (high-low)/low*100))

	sb.WriteString("\n请识别当前关键支撑位和阻力位。")

	return sb.String()
}

// parseAIKeyLevelResponseV2 解析 AI V2 格式响应
func parseAIKeyLevelResponseV2(response string) (*models.AIKeyLevelResultV2, error) {
	cleaned := extractJSON(response)

	var result models.AIKeyLevelResultV2
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return nil, fmt.Errorf("JSON解析失败: %w (raw: %s)", err, response)
	}

	// 验证和清理
	validResistances := make([]models.KeyLevelEntry, 0, len(result.Resistances))
	for _, entry := range result.Resistances {
		if entry.Price <= 0 {
			continue
		}
		if entry.Strength != models.LevelStrengthStrong &&
			entry.Strength != models.LevelStrengthMid &&
			entry.Strength != models.LevelStrengthWeak {
			entry.Strength = models.LevelStrengthMid
		}
		validResistances = append(validResistances, entry)
	}

	validSupports := make([]models.KeyLevelEntry, 0, len(result.Supports))
	for _, entry := range result.Supports {
		if entry.Price <= 0 {
			continue
		}
		if entry.Strength != models.LevelStrengthStrong &&
			entry.Strength != models.LevelStrengthMid &&
			entry.Strength != models.LevelStrengthWeak {
			entry.Strength = models.LevelStrengthMid
		}
		validSupports = append(validSupports, entry)
	}

	result.Resistances = validResistances
	result.Supports = validSupports
	return &result, nil
}
