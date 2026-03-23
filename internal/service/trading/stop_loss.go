package trading

import (
	"github.com/smallfire/starfire/internal/config"
	"github.com/smallfire/starfire/internal/models"
)

// StopLossStrategy 止盈止损策略
type StopLossStrategy struct {
	config *config.TradingConfig
}

// NewStopLossStrategy 创建止盈止损策略实例
func NewStopLossStrategy(cfg *config.TradingConfig) *StopLossStrategy {
	return &StopLossStrategy{config: cfg}
}

// CalculateStopLoss 计算止损价格
func (s *StopLossStrategy) CalculateStopLoss(entryPrice float64, direction string) float64 {
	if direction == "long" {
		return entryPrice * (1 - s.config.StopLossPercent)
	}
	return entryPrice * (1 + s.config.StopLossPercent)
}

// CalculateTakeProfit 计算止盈价格
func (s *StopLossStrategy) CalculateTakeProfit(entryPrice float64, direction string) float64 {
	if direction == "long" {
		return entryPrice * (1 + s.config.TakeProfitPercent)
	}
	return entryPrice * (1 - s.config.TakeProfitPercent)
}

// ShouldTriggerStopLoss 检查是否触发止损
func (s *StopLossStrategy) ShouldTriggerStopLoss(track *models.TradeTrack, currentPrice float64) bool {
	if track.Direction == "long" && track.StopLossPrice != nil {
		return currentPrice <= *track.StopLossPrice
	}
	if track.Direction == "short" && track.StopLossPrice != nil {
		return currentPrice >= *track.StopLossPrice
	}
	return false
}

// ShouldTriggerTakeProfit 检查是否触发止盈
func (s *StopLossStrategy) ShouldTriggerTakeProfit(track *models.TradeTrack, currentPrice float64) bool {
	if track.Direction == "long" && track.TakeProfitPrice != nil {
		return currentPrice >= *track.TakeProfitPrice
	}
	if track.Direction == "short" && track.TakeProfitPrice != nil {
		return currentPrice <= *track.TakeProfitPrice
	}
	return false
}
