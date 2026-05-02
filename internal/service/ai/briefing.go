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
	"github.com/smallfire/starfire/internal/service/notification"
	"github.com/smallfire/starfire/internal/service/trading"
	"go.uber.org/zap"
)

// BriefingService AI 每日市场简报服务
type BriefingService struct {
	client     *AIClient
	statsSvc   *trading.StatisticsService
	trackRepo  repository.TradeTrackRepo
	signalRepo repository.SignalRepo
	oppRepo    repository.OpportunityRepo
	trendRepo  repository.TrendRepo
	marketRepo repository.MarketRepo
	notifier   *notification.FeishuNotifier
	cfg        config.AIBriefingConfig
	stopCh     chan struct{}
	wg         sync.WaitGroup
	logger     *zap.Logger
}

// BriefingData 简报数据
type BriefingData struct {
	YesterdayStats      *trading.TradeStatistics
	OpenPositions       []*models.TradeTrack
	MarketTrends        map[string]trendSummary // marketCode -> summary
	RecentSignals       []*models.Signal
	ActiveOpportunities []*models.TradingOpportunity
}

type trendSummary struct {
	Bullish int
	Bearish int
	Sideways int
}

// briefingAIResponse AI 简报响应
type briefingAIResponse struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

// NewBriefingService 创建 AI 每日简报服务
func NewBriefingService(
	client *AIClient,
	statsSvc *trading.StatisticsService,
	trackRepo repository.TradeTrackRepo,
	signalRepo repository.SignalRepo,
	oppRepo repository.OpportunityRepo,
	trendRepo repository.TrendRepo,
	marketRepo repository.MarketRepo,
	notifier *notification.FeishuNotifier,
	cfg config.AIBriefingConfig,
	logger *zap.Logger,
) *BriefingService {
	return &BriefingService{
		client:     client,
		statsSvc:   statsSvc,
		trackRepo:  trackRepo,
		signalRepo: signalRepo,
		oppRepo:    oppRepo,
		trendRepo:  trendRepo,
		marketRepo: marketRepo,
		notifier:   notifier,
		cfg:        cfg,
		stopCh:     make(chan struct{}),
		logger:     logger,
	}
}

// Start 启动简报服务
func (s *BriefingService) Start() {
	s.wg.Add(1)
	go s.runLoop()
}

// Stop 停止简报服务
func (s *BriefingService) Stop() {
	close(s.stopCh)
	s.wg.Wait()
}

func (s *BriefingService) runLoop() {
	defer s.wg.Done()

	cstZone := time.FixedZone("CST", 8*3600)

	for {
		// 解析简报时间
		hour, minute, err := parseBriefingTime(s.cfg.Time)
		if err != nil {
			s.logger.Error("简报时间配置无效，默认08:00", zap.String("time", s.cfg.Time), zap.Error(err))
			hour, minute = 8, 0
		}

		// 计算下次触发时间
		now := time.Now().In(cstZone)
		target := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, cstZone)
		if !target.After(now) {
			target = target.Add(24 * time.Hour)
		}
		duration := target.Sub(now)

		s.logger.Info("AI简报下次触发时间",
			zap.String("target", target.Format("2006-01-02 15:04:05")),
			zap.Duration("wait", duration))

		select {
		case <-s.stopCh:
			s.logger.Info("AI简报服务停止")
			return
		case <-time.After(duration):
			s.generateBriefing()
		}
	}
}

func parseBriefingTime(timeStr string) (int, int, error) {
	var h, m int
	_, err := fmt.Sscanf(timeStr, "%d:%d", &h, &m)
	if err != nil {
		return 0, 0, err
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, 0, fmt.Errorf("invalid time: %s", timeStr)
	}
	return h, m, nil
}

func (s *BriefingService) generateBriefing() {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// 1. 收集数据
	data, err := s.collectBriefingData(ctx)
	if err != nil {
		s.logger.Error("收集简报数据失败", zap.Error(err))
		return
	}

	// 2. 构建 prompt
	systemPrompt := s.buildSystemPrompt()
	userPrompt := s.buildUserPrompt(data)

	// 3. 调用 AI
	messages := []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	resp, err := s.client.ChatCompletion(ctx, messages)
	if err != nil {
		s.logger.Error("AI简报生成失败", zap.Error(err))
		return
	}

	// 4. 解析响应
	jsonStr := extractJSON(resp)
	var result briefingAIResponse
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		s.logger.Error("解析AI简报响应失败", zap.Error(err), zap.String("raw", jsonStr))
		return
	}

	if result.Title == "" {
		result.Title = "星火量化 - AI 每日市场简报"
	}

	// 5. 发送通知
	if err := s.sendBriefing(result.Title, result.Content); err != nil {
		s.logger.Error("发送AI简报通知失败", zap.Error(err))
		return
	}

	s.logger.Info("AI每日简报发送成功", zap.String("title", result.Title))
}

func (s *BriefingService) collectBriefingData(_ context.Context) (*BriefingData, error) {
	cstZone := time.FixedZone("CST", 8*3600)
	now := time.Now().In(cstZone)
	yesterdayStart := time.Date(now.Year(), now.Month(), now.Day()-1, 0, 0, 0, 0, cstZone)
	yesterdayEnd := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, cstZone)
	start24hAgo := now.Add(-24 * time.Hour)

	data := &BriefingData{}

	// 昨日交易统计
	stats, err := s.statsSvc.GetStatistics(&yesterdayStart, &yesterdayEnd, "")
	if err != nil {
		s.logger.Warn("获取昨日交易统计失败", zap.Error(err))
	} else {
		data.YesterdayStats = stats
	}

	// 当前持仓
	positions, err := s.trackRepo.GetOpenPositions()
	if err != nil {
		s.logger.Warn("获取当前持仓失败", zap.Error(err))
	} else {
		data.OpenPositions = positions
	}

	// 市场趋势
	data.MarketTrends = make(map[string]trendSummary)
	markets, err := s.marketRepo.FindEnabled()
	if err != nil {
		s.logger.Warn("获取市场列表失败", zap.Error(err))
	} else {
		for _, m := range markets {
			trends, err := s.trendRepo.GetByMarket(m.MarketCode)
			if err != nil {
				continue
			}
			summary := trendSummary{}
			for _, t := range trends {
				switch t.TrendType {
				case "bullish":
					summary.Bullish++
				case "bearish":
					summary.Bearish++
				case "sideways":
					summary.Sideways++
				}
			}
			data.MarketTrends[m.MarketCode] = summary
		}
	}

	// 近24h信号（取前30条）
	signals, _, err := s.signalRepo.GetHistory(start24hAgo, now, 1, 30)
	if err != nil {
		s.logger.Warn("获取近24h信号失败", zap.Error(err))
	} else {
		data.RecentSignals = signals
	}

	// 活跃机会
	opps, err := s.oppRepo.GetActive()
	if err != nil {
		s.logger.Warn("获取活跃机会失败", zap.Error(err))
	} else {
		data.ActiveOpportunities = opps
	}

	return data, nil
}

func (s *BriefingService) buildSystemPrompt() string {
	return `你是量化交易系统的AI分析师。请根据提供的数据生成今日市场简报。

输出格式要求（纯JSON，不要markdown包裹）：
{"title":"简报标题(20字内)","content":"简报正文(markdown格式，500字内)"}

简报应包含以下板块，语言简洁专业：
1. 昨日交易回顾（平仓数、胜率、盈亏、亮点）
2. 当前持仓概况（持仓数、方向分布、浮盈浮亏）
3. 市场趋势扫描（各市场多空横盘分布）
4. 重要信号与机会（高强度信号、高评分机会）
5. 今日展望（基于数据的简短前瞻分析）

注意：
- 如果某个板块无数据，简要说明即可
- 盈亏数字保留2位小数
- 百分比保留1位小数
- 使用中文`
}

func (s *BriefingService) buildUserPrompt(data *BriefingData) string {
	cstZone := time.FixedZone("CST", 8*3600)
	now := time.Now().In(cstZone)
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("日期: %s (UTC+8)\n\n", now.Format("2006-01-02")))

	// 昨日交易统计
	sb.WriteString("## 昨日交易统计\n")
	if data.YesterdayStats != nil && data.YesterdayStats.TotalTrades > 0 {
		st := data.YesterdayStats
		sb.WriteString(fmt.Sprintf("- 平仓笔数: %d\n", st.TotalTrades))
		sb.WriteString(fmt.Sprintf("- 胜率: %.1f%% (%d胜%d负)\n", st.WinRate*100, st.WinTrades, st.LossTrades))
		sb.WriteString(fmt.Sprintf("- 总盈亏: %.2f USDT\n", st.TotalPnL))
		sb.WriteString(fmt.Sprintf("- 平均盈利: %.2f | 平均亏损: %.2f\n", st.AvgWin, st.AvgLoss))
		sb.WriteString(fmt.Sprintf("- 盈亏比: %.2f\n", st.ProfitFactor))
		sb.WriteString(fmt.Sprintf("- 最大回撤: %.2f%%\n", st.MaxDrawdownPct*100))
		if st.SharpeRatio != 0 {
			sb.WriteString(fmt.Sprintf("- 夏普比率: %.2f\n", st.SharpeRatio))
		}
	} else {
		sb.WriteString("- 昨日无平仓交易\n")
	}

	// 当前持仓
	sb.WriteString("\n## 当前持仓\n")
	if len(data.OpenPositions) > 0 {
		sb.WriteString(fmt.Sprintf("- 活跃持仓: %d\n", len(data.OpenPositions)))
		longCount, shortCount := 0, 0
		for _, t := range data.OpenPositions {
			if t.Direction == "long" {
				longCount++
			} else {
				shortCount++
			}
		}
		sb.WriteString(fmt.Sprintf("- 做多: %d | 做空: %d\n", longCount, shortCount))
		for i, t := range data.OpenPositions {
			if i >= 10 {
				sb.WriteString(fmt.Sprintf("  ... 还有 %d 个持仓\n", len(data.OpenPositions)-10))
				break
			}
			dirLabel := "做多"
			if t.Direction == "short" {
				dirLabel = "做空"
			}
			pnlStr := "N/A"
			if t.UnrealizedPnLPct != nil {
				pnlStr = fmt.Sprintf("%.1f%%", *t.UnrealizedPnLPct*100)
			}
			code := t.SymbolCode
			if code == "" {
				code = fmt.Sprintf("ID:%d", t.SymbolID)
			}
			sb.WriteString(fmt.Sprintf("  %d. %s %s 盈亏:%s\n", i+1, code, dirLabel, pnlStr))
		}
	} else {
		sb.WriteString("- 当前无持仓\n")
	}

	// 市场趋势
	sb.WriteString("\n## 市场趋势\n")
	if len(data.MarketTrends) > 0 {
		for marketCode, ts := range data.MarketTrends {
			total := ts.Bullish + ts.Bearish + ts.Sideways
			sb.WriteString(fmt.Sprintf("- %s: 多头%d | 空头%d | 横盘%d (共%d)\n",
				marketCode, ts.Bullish, ts.Bearish, ts.Sideways, total))
		}
	} else {
		sb.WriteString("- 无市场趋势数据\n")
	}

	// 近24h信号
	sb.WriteString("\n## 近24小时信号\n")
	if len(data.RecentSignals) > 0 {
		// 统计高强度信号
		var highStrength []*models.Signal
		for _, sig := range data.RecentSignals {
			if sig.Strength >= 3 {
				highStrength = append(highStrength, sig)
			}
		}
		sb.WriteString(fmt.Sprintf("- 信号总数: %d | 高强度(3星): %d\n",
			len(data.RecentSignals), len(highStrength)))
		for i, sig := range highStrength {
			if i >= 5 {
				sb.WriteString(fmt.Sprintf("  ... 还有 %d 个高强度信号\n", len(highStrength)-5))
				break
			}
			dirLabel := "做多"
			if sig.Direction == "short" {
				dirLabel = "做空"
			}
			sb.WriteString(fmt.Sprintf("  %d. %s %s %s 强度%d\n",
				i+1, sig.SymbolCode, sig.SignalType, dirLabel, sig.Strength))
		}
	} else {
		sb.WriteString("- 近24小时无信号\n")
	}

	// 活跃机会
	sb.WriteString("\n## 活跃交易机会\n")
	if len(data.ActiveOpportunities) > 0 {
		var highScore, midScore []*models.TradingOpportunity
		for _, opp := range data.ActiveOpportunities {
			if opp.Score >= 80 {
				highScore = append(highScore, opp)
			} else {
				midScore = append(midScore, opp)
			}
		}
		sb.WriteString(fmt.Sprintf("- 总数: %d | 高评分(>=80): %d | 中评分: %d\n",
			len(data.ActiveOpportunities), len(highScore), len(midScore)))
		for i, opp := range highScore {
			if i >= 3 {
				break
			}
			dirLabel := "做多"
			if opp.Direction == "short" {
				dirLabel = "做空"
			}
			sb.WriteString(fmt.Sprintf("  %d. %s %s 评分%d 含%d个信号\n",
				i+1, opp.SymbolCode, dirLabel, opp.Score, opp.SignalCount))
		}
	} else {
		sb.WriteString("- 当前无活跃机会\n")
	}

	return sb.String()
}

func (s *BriefingService) sendBriefing(title, content string) error {
	notifyContent := &notification.NotifyContent{
		Title:   title,
		Message: content,
		Type:    "briefing",
	}
	return s.notifier.Send(notifyContent)
}
