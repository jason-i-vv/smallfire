package ai

import (
	"github.com/smallfire/starfire/internal/models"
)

// AnalysisStep 单根 K 线的分析结果
type AnalysisStep struct {
	KlineTime  int64    `json:"kline_time"`
	ClosePrice float64  `json:"close_price"`
	Decision   string   `json:"decision"` // wait|alert|invalid|cooldown
	Confidence int      `json:"confidence"`
	Reasoning  string   `json:"reasoning"`

	// 交易参数
	EntryPrice    *float64 `json:"entry_price,omitempty"`
	StopLoss      *float64 `json:"stop_loss,omitempty"`
	TakeProfit    *float64 `json:"take_profit,omitempty"`
	RiskNotes     []string `json:"risk_notes"`

	// 策略特有字段
	TrendState    string                 `json:"trend_state,omitempty"`
	PullbackState string                 `json:"pullback_state,omitempty"`
	BuyPoint      string                 `json:"buy_point,omitempty"`
	Extra         map[string]interface{} `json:"extra,omitempty"`
}

// SkillInfo 策略摘要信息（给前端展示用）
type SkillInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Skill 分析策略接口
type Skill interface {
	Name() string
	Description() string
	SystemPrompt(marketCode string) string
	BuildFirstMessage(klines []models.Kline, observationStart int) string
	BuildIncrementalMessage(klines []models.Kline) string
	ParseResponse(raw string) ([]AnalysisStep, error)
}

// SkillRegistry 策略注册表
type SkillRegistry struct {
	skills map[string]Skill
}

// NewSkillRegistry 创建策略注册表
func NewSkillRegistry() *SkillRegistry {
	return &SkillRegistry{
		skills: make(map[string]Skill),
	}
}

// Register 注册策略
func (r *SkillRegistry) Register(skill Skill) {
	r.skills[skill.Name()] = skill
}

// Get 获取策略
func (r *SkillRegistry) Get(name string) (Skill, bool) {
	s, ok := r.skills[name]
	return s, ok
}

// List 返回所有已注册策略的信息
func (r *SkillRegistry) List() []SkillInfo {
	result := make([]SkillInfo, 0, len(r.skills))
	for _, s := range r.skills {
		result = append(result, SkillInfo{
			Name:        s.Name(),
			Description: s.Description(),
		})
	}
	return result
}
