package trading

import (
	"testing"

	"github.com/smallfire/starfire/internal/models"
)

func ptrFloat(f float64) *float64 { return &f }

func TestStopLossStrategy_CalculateStopLoss(t *testing.T) {
	cfg := newTestConfig()
	s := NewStopLossStrategy(cfg)

	t.Run("多头止损-低于入场价", func(t *testing.T) {
		sl := s.CalculateStopLoss(100, "long")
		expected := 100 * (1 - 0.02) // 98
		if sl != expected {
			t.Errorf("expected %f, got %f", expected, sl)
		}
	})

	t.Run("空头止损-高于入场价", func(t *testing.T) {
		sl := s.CalculateStopLoss(100, "short")
		expected := 100 * (1 + 0.02) // 102
		if sl != expected {
			t.Errorf("expected %f, got %f", expected, sl)
		}
	})
}

func TestStopLossStrategy_CalculateTakeProfit(t *testing.T) {
	cfg := newTestConfig()
	s := NewStopLossStrategy(cfg)

	t.Run("多头止盈-高于入场价", func(t *testing.T) {
		tp := s.CalculateTakeProfit(100, "long")
		expected := 100 * (1 + 0.05) // 105
		if tp != expected {
			t.Errorf("expected %f, got %f", expected, tp)
		}
	})

	t.Run("空头止盈-低于入场价", func(t *testing.T) {
		tp := s.CalculateTakeProfit(100, "short")
		expected := 100 * (1 - 0.05) // 95
		if tp != expected {
			t.Errorf("expected %f, got %f", expected, tp)
		}
	})
}

func TestStopLossStrategy_ShouldTrigger(t *testing.T) {
	cfg := newTestConfig()
	s := NewStopLossStrategy(cfg)

	t.Run("多头止损-刚好触及", func(t *testing.T) {
		track := &models.TradeTrack{Direction: "long", StopLossPrice: ptrFloat(98)}
		if !s.ShouldTriggerStopLoss(track, 98) {
			t.Error("should trigger stop loss at exactly stop price")
		}
	})

	t.Run("多头止损-高于止损价不触发", func(t *testing.T) {
		track := &models.TradeTrack{Direction: "long", StopLossPrice: ptrFloat(98)}
		if s.ShouldTriggerStopLoss(track, 98.01) {
			t.Error("should not trigger stop loss above stop price")
		}
	})

	t.Run("空头止损-刚好触及", func(t *testing.T) {
		track := &models.TradeTrack{Direction: "short", StopLossPrice: ptrFloat(102)}
		if !s.ShouldTriggerStopLoss(track, 102) {
			t.Error("should trigger stop loss at exactly stop price")
		}
	})

	t.Run("空头止盈-低于止盈价触发", func(t *testing.T) {
		track := &models.TradeTrack{Direction: "short", TakeProfitPrice: ptrFloat(95)}
		if !s.ShouldTriggerTakeProfit(track, 95) {
			t.Error("should trigger take profit at exactly target price")
		}
	})

	t.Run("nil止损价-不触发", func(t *testing.T) {
		track := &models.TradeTrack{Direction: "long", StopLossPrice: nil}
		if s.ShouldTriggerStopLoss(track, 0) {
			t.Error("should not trigger with nil stop loss price")
		}
	})

	t.Run("nil止盈价-不触发", func(t *testing.T) {
		track := &models.TradeTrack{Direction: "long", TakeProfitPrice: nil}
		if s.ShouldTriggerTakeProfit(track, 999) {
			t.Error("should not trigger with nil take profit price")
		}
	})
}
