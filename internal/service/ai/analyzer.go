package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/smallfire/starfire/internal/config"
	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/internal/repository"
	"go.uber.org/zap"
)

// AIJudgmentResult AI 判定结果
type AIJudgmentResult struct {
	Direction          string   `json:"direction"`
	Confidence         int      `json:"confidence"`
	Reasoning          string   `json:"reasoning"`
	KeyFactors         []string `json:"key_factors"`
	RiskWarnings       []string `json:"risk_warnings"`
	StrategySuggestion string   `json:"strategy_suggestion"`
	AnalyzedAt         string   `json:"analyzed_at"`
}

// 信号类型名称映射
var signalNameMap = map[string]string{
	"box_breakout": "箱体突破", "box_breakdown": "箱体跌破",
	"trend_retracement": "趋势回撤", "trend_reversal": "趋势反转",
	"resistance_break": "阻力位突破", "support_break": "支撑位跌破",
	"volume_surge": "量能放大", "price_surge_up": "价格急涨", "price_surge_down": "价格急跌",
	"volume_price_rise": "量价齐升", "volume_price_fall": "量价齐跌",
	"upper_wick_reversal": "上引线反转", "lower_wick_reversal": "下引线反转",
	"fake_breakout_upper": "假突破上引", "fake_breakout_lower": "假突破下引",
	"engulfing_bullish": "阳包阴吞没", "engulfing_bearish": "阴包阳吞没",
	"momentum_bullish": "连阳动量", "momentum_bearish": "连阴动量",
	"morning_star": "早晨之星", "evening_star": "黄昏之星",
}

// OpportunityAnalyzer 交易机会分析器
type OpportunityAnalyzer struct {
	client    *AIClient
	oppRepo   repository.OpportunityRepo
	klineRepo repository.KlineRepo
	judgeCfg  config.AIJudgeConfig
	cooldown  *CooldownTracker
	logger    *zap.Logger
	logDir    string
}

// NewOpportunityAnalyzer 创建交易机会分析器
func NewOpportunityAnalyzer(
	client *AIClient,
	oppRepo repository.OpportunityRepo,
	klineRepo repository.KlineRepo,
	judgeCfg config.AIJudgeConfig,
	cooldown *CooldownTracker,
	logger *zap.Logger,
	logDir string,
) *OpportunityAnalyzer {
	return &OpportunityAnalyzer{
		client:    client,
		oppRepo:   oppRepo,
		klineRepo: klineRepo,
		judgeCfg:  judgeCfg,
		cooldown:  cooldown,
		logger:    logger,
		logDir:    logDir,
	}
}

// OnOpportunity 实现 OpportunityHandler 接口，交易机会产生时自动调用 AI 分析
func (a *OpportunityAnalyzer) OnOpportunity(opp *models.TradingOpportunity) {
	if !a.judgeCfg.AutoAnalyze {
		return
	}

	// 检查冷却和每日限额
	if ok, reason := a.cooldown.CanAnalyze(opp.SymbolID); !ok {
		a.logger.Debug("AI 自动分析跳过",
			zap.String("symbol", opp.SymbolCode),
			zap.String("reason", reason))
		return
	}

	// 检查评分范围
	if opp.Score < a.judgeCfg.ScoreMin || opp.Score > a.judgeCfg.ScoreMax {
		return
	}

	// 记录调用
	a.cooldown.Record(opp.SymbolID)

	// 异步调用，不阻塞主流程
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		result, err := a.AnalyzeOpportunity(ctx, opp)
		if err != nil {
			a.logger.Error("AI 自动分析失败",
				zap.String("symbol", opp.SymbolCode),
				zap.Int("opportunity_id", opp.ID),
				zap.Error(err))
			return
		}
		a.logger.Info("AI 自动分析完成",
			zap.String("symbol", opp.SymbolCode),
			zap.Int("opportunity_id", opp.ID),
			zap.String("ai_direction", result.Direction),
			zap.Int("confidence", result.Confidence))
	}()
}

// AnalyzeOpportunity 分析交易机会
func (a *OpportunityAnalyzer) AnalyzeOpportunity(ctx context.Context, opp *models.TradingOpportunity) (*AIJudgmentResult, error) {
	// 获取近期K线数据作为上下文
	klineContext := a.fetchKlineContext(opp)

	// 构建消息
	systemPrompt := a.buildSystemPrompt()
	userMessage := a.buildUserPrompt(opp, klineContext)

	messages := []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userMessage},
	}

	a.logger.Info("调用 AI 分析",
		zap.String("symbol", opp.SymbolCode),
		zap.String("direction", opp.Direction),
		zap.Int("kline_context_lines", len(klineContext)),
	)

	// 调用 AI API
	response, err := a.client.ChatCompletion(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("AI API 调用失败: %w", err)
	}

	// 保存 AI 分析日志到文件
	if a.logDir != "" {
		a.saveAILog(opp, messages, response)
	}

	// 解析结果
	result, err := parseAIResponse(response)
	if err != nil {
		return nil, fmt.Errorf("解析 AI 响应失败: %w", err)
	}

	result.AnalyzedAt = time.Now().Format(time.RFC3339)

	// 保存到数据库
	if err := a.saveResult(opp, result); err != nil {
		a.logger.Error("保存 AI 判定结果失败", zap.Error(err))
	}

	return result, nil
}

// fetchKlineContext 获取近期K线数据作为分析上下文
func (a *OpportunityAnalyzer) fetchKlineContext(opp *models.TradingOpportunity) []models.Kline {
	if a.klineRepo == nil {
		return nil
	}

	klines, err := a.klineRepo.GetLastNPeriods(int64(opp.SymbolID), opp.Period, 30)
	if err != nil {
		a.logger.Warn("获取K线上下文失败", zap.Error(err))
		return nil
	}
	return klines
}

// buildSystemPrompt 构建系统提示词
func (a *OpportunityAnalyzer) buildSystemPrompt() string {
	return `你是交易信号分析器。只输出一个JSON对象，不要输出任何其他文字、分析或解释。

严格要求：直接输出JSON，不要用markdown代码块包裹。

输出格式：
{"direction":"long或short或neutral","confidence":0到100,"reasoning":"30字以内核心逻辑","key_factors":["因素1","因素2","因素3"],"risk_warnings":["风险1","风险2"],"strategy_suggestion":"50字以内建议"}`
}

// buildUserPrompt 构建用户消息（客观描述，不带方向引导）
func (a *OpportunityAnalyzer) buildUserPrompt(opp *models.TradingOpportunity, klines []models.Kline) string {
	var sb strings.Builder

	sb.WriteString("以下是量化系统检测到的一组交易信号，请评估其合理性：\n\n")

	// 基本信息
	sb.WriteString(fmt.Sprintf("标的: %s\n", opp.SymbolCode))
	sb.WriteString(fmt.Sprintf("周期: %s\n", opp.Period))

	// 信号检测时间
	signalTime := opp.LastSignalAt
	if signalTime == nil && opp.FirstSignalAt != nil {
		signalTime = opp.FirstSignalAt
	}
	if signalTime != nil {
		sb.WriteString(fmt.Sprintf("信号时间: %s (UTC)\n", signalTime.Format("2006-01-02 15:04")))
	}

	sb.WriteString(fmt.Sprintf("信号数量: %d\n\n", opp.SignalCount))

	// 信号列表（列举信号类型，不预设方向结论）
	sb.WriteString("检测到的信号:\n")
	for i, dir := range opp.ConfluenceDirections {
		parts := strings.SplitN(dir, ":", 2)
		signalType := parts[0]
		sigDir := ""
		if len(parts) > 1 {
			sigDir = parts[1]
		}
		name := signalNameMap[signalType]
		if name == "" {
			name = signalType
		}
		sigDirLabel := "看多"
		if sigDir == "short" {
			sigDirLabel = "看空"
		}
		sb.WriteString(fmt.Sprintf("  %d. %s (%s)\n", i+1, name, sigDirLabel))
	}

	// 系统拟定的交易策略（让 AI 评判是否合理）
	sb.WriteString("\n系统拟定的交易策略:\n")
	strategyDir := "做多"
	if opp.Direction == "short" {
		strategyDir = "做空"
	}
	sb.WriteString(fmt.Sprintf("  拟定方向: %s\n", strategyDir))
	if opp.SuggestedEntry != nil {
		sb.WriteString(fmt.Sprintf("  拟定入场价: %v\n", *opp.SuggestedEntry))
	}
	if opp.SuggestedStopLoss != nil {
		sb.WriteString(fmt.Sprintf("  拟定止损价: %v\n", *opp.SuggestedStopLoss))
	}
	if opp.SuggestedTakeProfit != nil {
		sb.WriteString(fmt.Sprintf("  拟定止盈目标: %v\n", *opp.SuggestedTakeProfit))
	}

	// 近期K线行情上下文
	// 注意：GetLastNPeriods 返回 DESC 顺序（klines[0] = 最新）
	if len(klines) > 0 {
		sb.WriteString("\n近期行情 (OHLCV + EMA):\n")
		// 取最近 20 根K线（slice 头部 = 最新），按时间正序输出便于阅读
		displayCount := 20
		if len(klines) < displayCount {
			displayCount = len(klines)
		}
		recentKlines := klines[:displayCount]
		// 反转为时间正序（从旧到新）
		for i := 0; i < len(recentKlines)/2; i++ {
			recentKlines[i], recentKlines[len(recentKlines)-1-i] = recentKlines[len(recentKlines)-1-i], recentKlines[i]
		}
		for _, k := range recentKlines {
			t := k.OpenTime.Format("01-02 15:04")
			change := ""
			if k.OpenPrice > 0 {
				change = fmt.Sprintf(" (%+.2f%%)", (k.ClosePrice-k.OpenPrice)/k.OpenPrice*100)
			}
			emaStr := ""
			if k.EMAShort != nil && k.EMAMedium != nil && k.EMALong != nil {
				emaStr = fmt.Sprintf(" EMA:%.4g/%.4g/%.4g", *k.EMAShort, *k.EMAMedium, *k.EMALong)
			}
			sb.WriteString(fmt.Sprintf("  %s O:%.4g H:%.4g L:%.4g C:%.4g Vol:%.0f%s%s\n",
				t, k.OpenPrice, k.HighPrice, k.LowPrice, k.ClosePrice, k.Volume, change, emaStr))
		}

		// EMA 趋势判断（取最新一根 K 线，即 klines[0]）
		latest := klines[0]
		if latest.EMAShort != nil && latest.EMAMedium != nil && latest.EMALong != nil {
			sb.WriteString(fmt.Sprintf("\n当前均线: EMA短=%.4f EMA中=%.4f EMA长=%.4f\n",
				*latest.EMAShort, *latest.EMAMedium, *latest.EMALong))
			if *latest.EMAShort > *latest.EMAMedium && *latest.EMAMedium > *latest.EMALong {
				sb.WriteString("均线状态: 多头排列 (短期>中期>长期)\n")
			} else if *latest.EMAShort < *latest.EMAMedium && *latest.EMAMedium < *latest.EMALong {
				sb.WriteString("均线状态: 空头排列 (短期<中期<长期)\n")
			} else {
				sb.WriteString("均线状态: 交叉/缠绕\n")
			}
		}

		// 近期高低点（基于显示的 K 线范围）
		var high, low float64
		for i, k := range recentKlines {
			if i == 0 || k.HighPrice > high {
				high = k.HighPrice
			}
			if i == 0 || k.LowPrice < low {
				low = k.LowPrice
			}
		}
		sb.WriteString(fmt.Sprintf("\n近期区间: 最高=%.4f 最低=%.4f 振幅=%.2f%%\n",
			high, low, (high-low)/low*100))
	}

	sb.WriteString("\n请综合技术分析，判断上述信号和策略是否合理，给出你的独立判断。")

	return sb.String()
}

// saveAILog 保存 AI 分析请求和响应到文件
func (a *OpportunityAnalyzer) saveAILog(opp *models.TradingOpportunity, messages []ChatMessage, response string) {
	if a.logDir == "" {
		return
	}

	// 确保日志目录存在
	if err := os.MkdirAll(a.logDir, 0755); err != nil {
		a.logger.Warn("创建 AI 日志目录失败", zap.String("dir", a.logDir), zap.Error(err))
		return
	}

	// 生成文件名: BTCUSDT_20260424_143052.json
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("%s_%s.json", opp.SymbolCode, timestamp)
	filePath := filepath.Join(a.logDir, filename)

	// 构建日志内容
	logData := map[string]interface{}{
		"symbol":       opp.SymbolCode,
		"symbol_id":    opp.SymbolID,
		"opp_id":       opp.ID,
		"direction":    opp.Direction,
		"period":       opp.Period,
		"score":        opp.Score,
		"request":      messages,
		"response":     response,
		"logged_at":    time.Now().Format(time.RFC3339),
	}

	jsonData, err := json.MarshalIndent(logData, "", "  ")
	if err != nil {
		a.logger.Warn("序列化 AI 日志失败", zap.Error(err))
		return
	}

	if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
		a.logger.Warn("写入 AI 日志文件失败", zap.String("file", filePath), zap.Error(err))
		return
	}

	a.logger.Info("AI 分析日志已保存", zap.String("file", filePath))
}

// saveResult 保存 AI 判定结果到数据库
func (a *OpportunityAnalyzer) saveResult(opp *models.TradingOpportunity, result *AIJudgmentResult) error {
	resultMap := map[string]any{
		"direction":           result.Direction,
		"confidence":          result.Confidence,
		"reasoning":           result.Reasoning,
		"key_factors":         result.KeyFactors,
		"risk_warnings":       result.RiskWarnings,
		"strategy_suggestion": result.StrategySuggestion,
		"analyzed_at":         result.AnalyzedAt,
	}
	jsonb := models.JSONB(resultMap)
	opp.AIJudgment = &jsonb

	opp.AIAdjustment = calculateAdjustment(opp.Direction, result)

	if err := a.oppRepo.Update(opp); err != nil {
		return err
	}

	a.logger.Info("AI 判定结果已保存",
		zap.Int("id", opp.ID),
		zap.String("symbol", opp.SymbolCode),
		zap.String("ai_direction", result.Direction),
		zap.Int("confidence", result.Confidence),
		zap.Int("adjustment", opp.AIAdjustment),
	)

	return nil
}

// calculateAdjustment 计算评分调整
func calculateAdjustment(signalDirection string, aiResult *AIJudgmentResult) int {
	if aiResult.Direction == signalDirection {
		return int(float64(aiResult.Confidence) * 0.15)
	} else if aiResult.Direction == "neutral" {
		return -5
	}
	return -int(float64(aiResult.Confidence) * 0.15)
}

// parseAIResponse 解析 AI 响应
func parseAIResponse(response string) (*AIJudgmentResult, error) {
	cleaned := extractJSON(response)

	var result AIJudgmentResult
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %w (raw: %s)", err, response)
	}

	// 字段验证
	if result.Direction == "" || (result.Direction != "long" && result.Direction != "short" && result.Direction != "neutral") {
		result.Direction = "neutral"
	}
	if result.Confidence < 0 {
		result.Confidence = 0
	}
	if result.Confidence > 100 {
		result.Confidence = 100
	}
	if result.Reasoning == "" {
		result.Reasoning = "分析完成"
	}
	if len(result.Reasoning) > 2000 {
		result.Reasoning = result.Reasoning[:2000]
	}
	if result.KeyFactors == nil {
		result.KeyFactors = []string{}
	}
	if len(result.KeyFactors) > 20 {
		result.KeyFactors = result.KeyFactors[:20]
	}
	if result.RiskWarnings == nil {
		result.RiskWarnings = []string{}
	}
	if len(result.RiskWarnings) > 20 {
		result.RiskWarnings = result.RiskWarnings[:20]
	}
	if len(result.StrategySuggestion) > 1000 {
		result.StrategySuggestion = result.StrategySuggestion[:1000]
	}

	return &result, nil
}

// extractJSON 从 AI 响应中提取 JSON
func extractJSON(s string) string {
	re := regexp.MustCompile("(?s)```(?:json)?\\s*\\n?(.*?)\\n?```")
	matches := re.FindStringSubmatch(s)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}

	if start >= 0 {
		return fixIncompleteJSON(s[start:])
	}

	return strings.TrimSpace(s)
}

// fixIncompleteJSON 尝试修复不完整的 JSON
func fixIncompleteJSON(s string) string {
	braceCount := 0
	bracketCount := 0
	inString := false
	escape := false

	for _, c := range s {
		if escape {
			escape = false
			continue
		}
		if c == '\\' {
			escape = true
			continue
		}
		if c == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		if c == '{' {
			braceCount++
		} else if c == '}' {
			braceCount--
		} else if c == '[' {
			bracketCount++
		} else if c == ']' {
			bracketCount--
		}
	}

	result := s

	if inString {
		lastQuote := strings.LastIndex(result, "\"")
		if lastQuote > 0 {
			beforeQuote := result[:lastQuote]
			lastColon := strings.LastIndex(beforeQuote, ":")
			lastComma := strings.LastIndex(beforeQuote, ",")
			cutAt := lastComma
			if lastColon > lastComma {
				cutAt = lastColon - 1
				keyStart := strings.LastIndex(beforeQuote[:lastColon], "\"")
				if keyStart >= 0 {
					cutAt = keyStart - 1
				}
			}
			if cutAt > 0 {
				result = result[:cutAt]
			}
		}
	}

	for bracketCount > 0 {
		result += "]"
		bracketCount--
	}
	for braceCount > 0 {
		result += "}"
		braceCount--
	}

	return result
}
