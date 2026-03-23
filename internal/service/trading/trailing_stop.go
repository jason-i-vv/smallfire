package trading

import (
	"github.com/smallfire/starfire/internal/config"
	"github.com/smallfire/starfire/internal/models"
)

// TrailingStopStrategy 移动止损策略
type TrailingStopStrategy struct {
	config *config.TradingConfig
}

// TrailingState 移动止损状态
type TrailingState struct {
	IsActivated     bool
	ActivationPrice float64 // 激活价格
	HighestPrice    float64 // 激活后的最高价(多头)
	LowestPrice     float64 // 激活后的最低价(空头)
	CurrentStop     float64 // 当前止损价
}

// NewTrailingStopStrategy 创建移动止损策略实例
func NewTrailingStopStrategy(cfg *config.TradingConfig) *TrailingStopStrategy {
	return &TrailingStopStrategy{config: cfg}
}

// CheckAndUpdate 检查并更新移动止损
func (s *TrailingStopStrategy) CheckAndUpdate(track *models.TradeTrack, currentPrice float64, state *TrailingState) *TrailingState {
	if !s.config.TrailingStopEnabled {
		return state
	}

	// 获取入场价格
	var entryPrice float64
	if track.EntryPrice != nil {
		entryPrice = *track.EntryPrice
	} else {
		return state
	}

	activationPrice := entryPrice
	if track.TrailingActivationPct != nil {
		if track.Direction == "long" {
			activationPrice = entryPrice * (1 + *track.TrailingActivationPct)
		} else {
			activationPrice = entryPrice * (1 - *track.TrailingActivationPct)
		}
	}

	if track.Direction == "long" {
		// 多头逻辑
		if !state.IsActivated && currentPrice >= activationPrice {
			state.IsActivated = true
			state.ActivationPrice = activationPrice
			state.HighestPrice = currentPrice
			// 初始止损
			state.CurrentStop = currentPrice * (1 - s.config.TrailingDistance)
		}

		if state.IsActivated {
			if currentPrice > state.HighestPrice {
				state.HighestPrice = currentPrice
				// 逐步上移止损
				newStop := currentPrice * (1 - s.config.TrailingDistance)
				if newStop > state.CurrentStop {
					state.CurrentStop = newStop
				}
			}

			// 检查是否触发移动止损
			if currentPrice <= state.CurrentStop {
				return state // 返回触发状态
			}
		}
	} else {
		// 空头逻辑
		if !state.IsActivated && currentPrice <= activationPrice {
			state.IsActivated = true
			state.ActivationPrice = activationPrice
			state.LowestPrice = currentPrice
			state.CurrentStop = currentPrice * (1 + s.config.TrailingDistance)
		}

		if state.IsActivated {
			if currentPrice < state.LowestPrice {
				state.LowestPrice = currentPrice
				newStop := currentPrice * (1 + s.config.TrailingDistance)
				if newStop < state.CurrentStop {
					state.CurrentStop = newStop
				}
			}

			if currentPrice >= state.CurrentStop {
				return state // 返回触发状态
			}
		}
	}

	return state
}
