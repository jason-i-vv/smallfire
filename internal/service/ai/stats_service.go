package ai

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/smallfire/starfire/internal/database"
	"github.com/smallfire/starfire/internal/models"
)

// AIStatsService AI 分析统计服务
type AIStatsService struct {
	db *database.DB
}

// NewAIStatsService 创建 AI 统计服务
func NewAIStatsService(db *database.DB) *AIStatsService {
	return &AIStatsService{db: db}
}

// --- 返回数据结构 ---

type DailyAICount struct {
	Date    string `json:"date"`
	Total   int    `json:"total"`
	Success int    `json:"success"`
}

type AIOverview struct {
	TotalCalls    int            `json:"total_calls"`
	AvgConfidence float64        `json:"avg_confidence"`
	AgreeRate     float64        `json:"agree_rate"`
	DirectionDist map[string]int `json:"direction_dist"`
}

type AIAccuracyAnalysis struct {
	TotalWithTrade      int     `json:"total_with_trade"`
	AIWinRate           float64 `json:"ai_win_rate"`
	AgreeWinRate        float64 `json:"agree_win_rate"`
	DisagreeWinRate     float64 `json:"disagree_win_rate"`
	AgreeTotalTrades    int     `json:"agree_total_trades"`
	AgreeWinTrades      int     `json:"agree_win_trades"`
	DisagreeTotalTrades int     `json:"disagree_total_trades"`
	DisagreeWinTrades   int     `json:"disagree_win_trades"`
}

type AIDirectionStat struct {
	AIDirection   string  `json:"ai_direction"`
	TotalCalls    int     `json:"total_calls"`
	AvgConfidence float64 `json:"avg_confidence"`
	WinRate       float64 `json:"win_rate"`
	TotalPnL      float64 `json:"total_pnl"`
}

type AIConfidenceAnalysis struct {
	HighConfidence   AIConfidenceBucket `json:"high_confidence"`
	MediumConfidence AIConfidenceBucket `json:"medium_confidence"`
	LowConfidence    AIConfidenceBucket `json:"low_confidence"`
}

type AIConfidenceBucket struct {
	Count   int     `json:"count"`
	WinRate float64 `json:"win_rate"`
	AvgPnL  float64 `json:"avg_pnl"`
}

// --- 内部辅助结构 ---

type aiJudgmentRow struct {
	oppID      int
	oppDir     string
	aiDir      string
	confidence int
	createdAt  time.Time
}

type tradeRow struct {
	oppID *int
	pnl   *float64
}

// --- 公开方法 ---

// GetDailyCallStats 每日 AI 调用统计
func (s *AIStatsService) GetDailyCallStats(startDate, endDate *time.Time) ([]DailyAICount, error) {
	query := `SELECT DATE(created_at)::text AS date, COUNT(*) AS total,
		COUNT(*) FILTER (WHERE ai_judgment IS NOT NULL) AS success
		FROM trading_opportunities
		WHERE ai_judgment IS NOT NULL`

	args := []any{}
	if startDate != nil {
		query += fmt.Sprintf(` AND created_at >= $%d`, len(args)+1)
		args = append(args, *startDate)
	}
	if endDate != nil {
		query += fmt.Sprintf(` AND created_at <= $%d`, len(args)+1)
		args = append(args, *endDate)
	}
	query += ` GROUP BY DATE(created_at) ORDER BY DATE(created_at)`

	rows, err := s.db.Query(context.Background(), query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询每日调用统计失败: %w", err)
	}
	defer rows.Close()

	var result []DailyAICount
	for rows.Next() {
		var item DailyAICount
		if err := rows.Scan(&item.Date, &item.Total, &item.Success); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

// GetOverview AI 分析概览
func (s *AIStatsService) GetOverview(startDate, endDate *time.Time) (*AIOverview, error) {
	judgments, err := s.fetchJudgments(startDate, endDate)
	if err != nil {
		return nil, err
	}

	overview := &AIOverview{
		TotalCalls:    len(judgments),
		DirectionDist: make(map[string]int),
	}
	if len(judgments) == 0 {
		return overview, nil
	}

	var totalConf int
	agreeCount := 0
	for _, j := range judgments {
		totalConf += j.confidence
		overview.DirectionDist[j.aiDir]++
		if j.aiDir == j.oppDir {
			agreeCount++
		}
	}
	overview.AvgConfidence = float64(totalConf) / float64(len(judgments))
	overview.AgreeRate = float64(agreeCount) / float64(len(judgments))
	return overview, nil
}

// GetAccuracyAnalysis AI 准确率分析
func (s *AIStatsService) GetAccuracyAnalysis(startDate, endDate *time.Time) (*AIAccuracyAnalysis, error) {
	judgments, err := s.fetchJudgments(startDate, endDate)
	if err != nil {
		return nil, err
	}

	tradeMap, err := s.buildTradeMap()
	if err != nil {
		return nil, err
	}

	result := &AIAccuracyAnalysis{}
	var aiCorrect, aiTotal int
	var agreeWin, agreeTotal int
	var disagreeWin, disagreeTotal int

	for _, j := range judgments {
		pnl, ok := tradeMap[j.oppID]
		if !ok {
			continue
		}
		result.TotalWithTrade++
		isAgree := j.aiDir == j.oppDir

		if (isAgree && pnl > 0) || (!isAgree && pnl <= 0) {
			aiCorrect++
		}
		aiTotal++

		if isAgree {
			agreeTotal++
			if pnl > 0 {
				agreeWin++
			}
		} else {
			disagreeTotal++
			if pnl <= 0 {
				disagreeWin++
			}
		}
	}

	if aiTotal > 0 {
		result.AIWinRate = float64(aiCorrect) / float64(aiTotal)
	}
	result.AgreeTotalTrades = agreeTotal
	result.AgreeWinTrades = agreeWin
	if agreeTotal > 0 {
		result.AgreeWinRate = float64(agreeWin) / float64(agreeTotal)
	}
	result.DisagreeTotalTrades = disagreeTotal
	result.DisagreeWinTrades = disagreeWin
	if disagreeTotal > 0 {
		result.DisagreeWinRate = float64(disagreeWin) / float64(disagreeTotal)
	}
	return result, nil
}

// GetDirectionStats 按方向统计
func (s *AIStatsService) GetDirectionStats(startDate, endDate *time.Time) ([]AIDirectionStat, error) {
	judgments, err := s.fetchJudgments(startDate, endDate)
	if err != nil {
		return nil, err
	}

	tradeMap, err := s.buildTradeMap()
	if err != nil {
		return nil, err
	}

	type agg struct {
		count, confSum, winCount, tradeCount int
		pnlSum                               float64
	}
	groups := make(map[string]*agg)
	for _, j := range judgments {
		if _, ok := groups[j.aiDir]; !ok {
			groups[j.aiDir] = &agg{}
		}
		g := groups[j.aiDir]
		g.count++
		g.confSum += j.confidence
		if pnl, ok := tradeMap[j.oppID]; ok {
			g.tradeCount++
			g.pnlSum += pnl
			if pnl > 0 {
				g.winCount++
			}
		}
	}

	var result []AIDirectionStat
	for dir, g := range groups {
		stat := AIDirectionStat{AIDirection: dir, TotalCalls: g.count}
		if g.count > 0 {
			stat.AvgConfidence = float64(g.confSum) / float64(g.count)
		}
		if g.tradeCount > 0 {
			stat.WinRate = float64(g.winCount) / float64(g.tradeCount)
			stat.TotalPnL = g.pnlSum
		}
		result = append(result, stat)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].TotalCalls > result[j].TotalCalls })
	return result, nil
}

// GetConfidenceAnalysis 置信度分析
func (s *AIStatsService) GetConfidenceAnalysis(startDate, endDate *time.Time) (*AIConfidenceAnalysis, error) {
	judgments, err := s.fetchJudgments(startDate, endDate)
	if err != nil {
		return nil, err
	}

	tradeMap, err := s.buildTradeMap()
	if err != nil {
		return nil, err
	}

	type bucket struct{ count, winCount, tradeCount int; pnlSum float64 }
	high, medium, low := &bucket{}, &bucket{}, &bucket{}

	for _, j := range judgments {
		var b *bucket
		switch {
		case j.confidence >= 70:
			b = high
		case j.confidence >= 40:
			b = medium
		default:
			b = low
		}
		b.count++
		if pnl, ok := tradeMap[j.oppID]; ok {
			b.tradeCount++
			b.pnlSum += pnl
			if pnl > 0 {
				b.winCount++
			}
		}
	}

	convert := func(b *bucket) AIConfidenceBucket {
		bk := AIConfidenceBucket{Count: b.count}
		if b.tradeCount > 0 {
			bk.WinRate = float64(b.winCount) / float64(b.tradeCount)
			bk.AvgPnL = b.pnlSum / float64(b.tradeCount)
		}
		return bk
	}

	return &AIConfidenceAnalysis{
		HighConfidence:   convert(high),
		MediumConfidence: convert(medium),
		LowConfidence:    convert(low),
	}, nil
}

// --- 内部方法 ---

func (s *AIStatsService) fetchJudgments(startDate, endDate *time.Time) ([]aiJudgmentRow, error) {
	query := `SELECT id, direction, ai_judgment->>'direction' AS ai_dir,
		(ai_judgment->>'confidence')::int AS conf, created_at
		FROM trading_opportunities WHERE ai_judgment IS NOT NULL`

	args := []any{}
	if startDate != nil {
		query += fmt.Sprintf(` AND created_at >= $%d`, len(args)+1)
		args = append(args, *startDate)
	}
	if endDate != nil {
		query += fmt.Sprintf(` AND created_at <= $%d`, len(args)+1)
		args = append(args, *endDate)
	}
	query += ` ORDER BY created_at`

	rows, err := s.db.Query(context.Background(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []aiJudgmentRow
	for rows.Next() {
		var id int
		var direction, aiDir string
		var conf int
		var createdAt time.Time
		if err := rows.Scan(&id, &direction, &aiDir, &conf, &createdAt); err != nil {
			return nil, err
		}
		if aiDir == "" {
			continue
		}

		result = append(result, aiJudgmentRow{
			oppID: id, oppDir: direction, aiDir: aiDir,
			confidence: conf, createdAt: createdAt,
		})
	}
	return result, rows.Err()
}

// buildTradeMap 返回 opportunity_id -> pnl 映射
func (s *AIStatsService) buildTradeMap() (map[int]float64, error) {
	rows, err := s.db.Query(context.Background(),
		`SELECT opportunity_id, pnl FROM trade_tracks
		 WHERE opportunity_id IS NOT NULL AND status = $1 AND pnl IS NOT NULL`,
		models.TrackStatusClosed)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	m := make(map[int]float64)
	for rows.Next() {
		var oppID int
		var pnl float64
		if err := rows.Scan(&oppID, &pnl); err != nil {
			return nil, err
		}
		m[oppID] = pnl
	}
	return m, rows.Err()
}
