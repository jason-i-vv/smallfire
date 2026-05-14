package scoring

import (
	"math"

	"github.com/smallfire/starfire/internal/models"
)

// ScoringWeights 评分权重配置
type ScoringWeights struct {
	StrategyWinRate float64 `mapstructure:"strategy_win_rate" json:"strategy_win_rate"` // 策略历史胜率 40%
	MultiConfluence float64 `mapstructure:"multi_confluence" json:"multi_confluence"`   // 多策略共识   25%
	SignalStrength  float64 `mapstructure:"signal_strength" json:"signal_strength"`     // 信号强度     10%
	VolumeConfirm   float64 `mapstructure:"volume_confirm" json:"volume_confirm"`       // 成交量确认   15%
	MarketRegime    float64 `mapstructure:"market_regime" json:"market_regime"`         // 市场状态匹配 10%
}

// DefaultWeights 默认权重（优化后：胜率权重提升到40%，信号强度降到10%）
var DefaultWeights = ScoringWeights{
	StrategyWinRate: 0.40,
	MultiConfluence: 0.25,
	SignalStrength:  0.10,
	VolumeConfirm:   0.15,
	MarketRegime:    0.10,
}

// ScoringContext 评分上下文（外部传入的原始数据）
type ScoringContext struct {
	// 策略历史胜率
	WinRate float64 // 0.0 ~ 1.0

	// 多策略共识
	ConfluenceCount    int     // 同方向的信号数量
	TotalActiveSignals int     // 当前所有活跃信号数量
	ConfluenceRatio    float64 // 共识比例 (confluenceCount / totalActiveSignals)

	// 信号强度（来自 Signal.Strength，1-5）
	Strength int

	// 成交量确认
	VolumeRatio float64 // 当前成交量 / N周期均量 (如 1.5 = 超均量50%)

	// 市场状态匹配
	MarketRegime     string  // "trending" / "ranging" / "volatile"
	RegimeMatchScore float64 // 策略与当前市场状态的匹配度 0.0 ~ 1.0

	// 交易方向（用于做空惩罚）
	Direction string // "long" / "short"
}

// SignalScorer 信号评分引擎
type SignalScorer struct {
	weights ScoringWeights
}

// NewSignalScorer 创建评分引擎
func NewSignalScorer(weights ScoringWeights) *SignalScorer {
	if weights.StrategyWinRate+weights.MultiConfluence+weights.SignalStrength+weights.VolumeConfirm+weights.MarketRegime == 0 {
		weights = DefaultWeights
	}
	return &SignalScorer{weights: weights}
}

// Score 评分入口 - 对单个信号进行评分
func (s *SignalScorer) Score(signal *models.Signal, ctx *ScoringContext) *models.ScoreResult {
	if ctx == nil {
		ctx = &ScoringContext{}
	}

	dimensions := models.ScoreDimensions{
		StrategyWinRate: s.calcStrategyWinRate(ctx.WinRate),
		MultiConfluence: s.calcMultiConfluence(ctx.ConfluenceCount, ctx.TotalActiveSignals, ctx.ConfluenceRatio),
		SignalStrength:  s.calcSignalStrength(ctx.Strength),
		VolumeConfirm:   s.calcVolumeConfirm(ctx.VolumeRatio),
		MarketRegime:    s.calcMarketRegime(ctx.RegimeMatchScore),
	}

	totalScore := s.weightedSum(dimensions)

	// 钳制到 0-100
	totalScore = clamp(totalScore, 0, 100)

	breakdown := map[string]interface{}{
		"win_rate_input":    ctx.WinRate,
		"confluence_count":  ctx.ConfluenceCount,
		"total_signals":     ctx.TotalActiveSignals,
		"confluence_ratio":  ctx.ConfluenceRatio,
		"strength_input":    ctx.Strength,
		"volume_ratio":      ctx.VolumeRatio,
		"regime":            ctx.MarketRegime,
		"regime_match":      ctx.RegimeMatchScore,
		"direction":         ctx.Direction,
		"weights":           s.weights,
	}

	return &models.ScoreResult{
		TotalScore: totalScore,
		Dimensions: dimensions,
		Breakdown:  breakdown,
	}
}

// ScoreOpportunity 对交易机会（多信号聚合）评分
func (s *SignalScorer) ScoreOpportunity(signals []*models.Signal, ctx *ScoringContext) *models.ScoreResult {
	if len(signals) == 0 || ctx == nil {
		return &models.ScoreResult{TotalScore: 0}
	}

	// 聚合多个信号的强度
	avgStrength := 0
	maxStrength := 0
	for _, sig := range signals {
		avgStrength += sig.Strength
		if sig.Strength > maxStrength {
			maxStrength = sig.Strength
		}
	}
	avgStrength = avgStrength / len(signals)

	// 用聚合后的强度覆盖上下文
	ctx.Strength = maxStrength
	if ctx.ConfluenceCount == 0 {
		ctx.ConfluenceCount = len(signals)
	}
	if ctx.TotalActiveSignals == 0 {
		ctx.TotalActiveSignals = len(signals)
	}

	return s.Score(signals[0], ctx)
}

// calcStrategyWinRate 策略历史胜率 → 0-100 分
// 线性映射：胜率直接对应分数，让评分准确反映胜率
// 42.5% → 42, 50% → 50, 77% → 77
func (s *SignalScorer) calcStrategyWinRate(winRate float64) int {
	if winRate <= 0 {
		return 50 // 无历史数据时给中性分，不惩罚
	}
	score := int(winRate * 100)
	return clamp(score, 0, 100)
}

// calcMultiConfluence 多策略共识 → 0-100 分
// 基于共识比例线性评分，1个信号适度降分
func (s *SignalScorer) calcMultiConfluence(confluenceCount, totalSignals int, ratio float64) int {
	if totalSignals == 0 {
		return 50 // 无其他信号参考时给中性分
	}

	if ratio <= 0 {
		ratio = float64(confluenceCount) / float64(totalSignals)
	}

	// 基于共识比例评分：ratio 0.0~1.0 → 0~100
	// 单信号(confluenceCount=1)适度降分：上限60分
	if confluenceCount <= 1 {
		return clamp(int(ratio*60), 0, 60)
	}

	return clamp(int(ratio*100), 0, 100)
}

// calcSignalStrength 信号强度(1-5) → 0-100 分
func (s *SignalScorer) calcSignalStrength(strength int) int {
	if strength <= 0 {
		return 20
	}
	// 1→20, 2→40, 3→60, 4→80, 5→100
	return strength * 20
}

// calcVolumeConfirm 成交量确认 → 0-100 分
func (s *SignalScorer) calcVolumeConfirm(volumeRatio float64) int {
	if volumeRatio <= 0 {
		return 50 // 无数据时给中性分，不惩罚
	}

	var score int
	switch {
	case volumeRatio < 0.5:
		score = 20 // 缩量
	case volumeRatio < 1.0:
		score = 40 // 偏低
	case volumeRatio < 2.0:
		score = 60 // 正常
	case volumeRatio < 5.0:
		score = 80 // 放量确认
	case volumeRatio <= 10.0:
		score = 95 // 强放量
	default:
		// >10x 异常放量，适度降分（可能清算/操纵，但仍确认了方向性）
		score = 80
	}
	return score
}

// calcMarketRegime 市场状态匹配 → 0-100 分
func (s *SignalScorer) calcMarketRegime(matchScore float64) int {
	if matchScore <= 0 {
		return 50 // 无数据时给中间分
	}
	return int(matchScore * 100)
}

// weightedSum 加权求和
func (s *SignalScorer) weightedSum(d models.ScoreDimensions) int {
	return int(math.Round(
		float64(d.StrategyWinRate)*s.weights.StrategyWinRate +
			float64(d.MultiConfluence)*s.weights.MultiConfluence +
			float64(d.SignalStrength)*s.weights.SignalStrength +
			float64(d.VolumeConfirm)*s.weights.VolumeConfirm +
			float64(d.MarketRegime)*s.weights.MarketRegime,
	))
}

func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
