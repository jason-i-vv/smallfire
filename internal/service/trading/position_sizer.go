package trading

import (
	"math"

	"github.com/smallfire/starfire/internal/config"
)

// PositionSizer 仓位计算服务
type PositionSizer struct {
	config  *config.TradingConfig
	capital float64 // 当前权益
}

// NewPositionSizer 创建仓位计算器实例
func NewPositionSizer(cfg *config.TradingConfig) *PositionSizer {
	return &PositionSizer{
		config:  cfg,
		capital: cfg.InitialCapital,
	}
}

// CalculatePosition 根据风险金额计算仓位
func (s *PositionSizer) CalculatePosition(entryPrice, stopLossPrice float64) (quantity, positionValue float64) {
	// 风险金额 = 账户 * 风险比例
	riskAmount := s.capital * s.config.MaxLossPerTrade

	// 每单位价格风险
	riskPerUnit := math.Abs(entryPrice - stopLossPrice)

	// 数量
	quantity = riskAmount / riskPerUnit

	// 仓位价值
	positionValue = quantity * entryPrice

	// 限制最大仓位
	maxPosition := s.capital * s.config.PositionSize
	if positionValue > maxPosition {
		quantity = maxPosition / entryPrice
		positionValue = maxPosition
	}

	return
}

// UpdateCapital 更新账户权益
func (s *PositionSizer) UpdateCapital(pnl float64) {
	s.capital += pnl
}

// GetCapital 获取当前权益
func (s *PositionSizer) GetCapital() float64 {
	return s.capital
}
