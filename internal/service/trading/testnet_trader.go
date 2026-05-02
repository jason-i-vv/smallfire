package trading

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/smallfire/starfire/internal/config"
	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/internal/repository"
	"go.uber.org/zap"
)

// TestnetTrader Bybit Testnet 模拟交易服务
// 实现 OpportunityHandler 接口，通过 Bybit Testnet API 真实下单
type TestnetTrader struct {
	config     *config.TradingConfig
	trackRepo  repository.TradeTrackRepo
	klineRepo  repository.KlineRepo
	client     *BybitTradingClient
	logger     *zap.Logger
	mu         sync.Mutex
}

// NewTestnetTrader 创建 Bybit Testnet 交易服务
func NewTestnetTrader(
	cfg *config.TradingConfig,
	trackRepo repository.TradeTrackRepo,
	klineRepo repository.KlineRepo,
	logger *zap.Logger,
) *TestnetTrader {
	testnetCfg := cfg.Testnet
	client := NewBybitTradingClient(
		testnetCfg.BaseURL,
		testnetCfg.APIKey,
		testnetCfg.APISecret,
		testnetCfg.RecvWindow,
		logger,
	)

	return &TestnetTrader{
		config:    cfg,
		trackRepo: trackRepo,
		klineRepo: klineRepo,
		client:    client,
		logger:    logger,
	}
}

// GetClient 获取 Bybit 交易客户端（供 TestnetPositionMonitor 使用）
func (t *TestnetTrader) GetClient() *BybitTradingClient {
	return t.client
}

// OnOpportunity 当交易机会产生时调用
func (t *TestnetTrader) OnOpportunity(opp *models.TradingOpportunity) {
	t.mu.Lock()
	defer t.mu.Unlock()

	testnetCfg := t.config.Testnet

	// 1. 检查是否启用
	if !testnetCfg.Enabled {
		return
	}

	// 2. 检查评分阈值
	if opp.Score < testnetCfg.ScoreThreshold {
		return
	}

	// 3. 检查该机会是否已有 testnet 持仓
	if opp.ID > 0 {
		existing, err := t.trackRepo.GetOpenByOpportunityIDAndSource(opp.ID, models.TradeSourceTestnet)
		if err != nil {
			t.logger.Error("[Testnet] 查询已有持仓失败", zap.Int("opportunity_id", opp.ID), zap.Error(err))
			return
		}
		if existing != nil {
			t.logger.Debug("[Testnet] 该机会已有持仓，跳过",
				zap.Int("opportunity_id", opp.ID),
				zap.Int("existing_track_id", existing.ID))
			return
		}
	}

	// 4. 检查最大持仓数
	openTracks, err := t.trackRepo.GetOpenBySource(models.TradeSourceTestnet)
	if err != nil {
		t.logger.Error("[Testnet] 查询 testnet 持仓失败", zap.Error(err))
		return
	}
	if len(openTracks) >= testnetCfg.MaxOpenPositions {
		t.logger.Debug("[Testnet] 已达最大持仓数，跳过",
			zap.Int("current", len(openTracks)),
			zap.Int("max", testnetCfg.MaxOpenPositions))
		return
	}

	// 5. 检查 Bybit 是否已有同交易对同方向的持仓
	existingPos, err := t.client.QueryPosition(opp.SymbolCode)
	if err != nil {
		t.logger.Warn("[Testnet] 查询 Bybit 持仓失败",
			zap.String("symbol", opp.SymbolCode),
			zap.Error(err))
	}
	if existingPos != nil && existingPos.Size != "0" {
		// 检查方向是否一致
		existingSide := existingPos.Side // "Buy" or "Sell"
		requiredSide := "Buy"
		if opp.Direction == models.DirectionShort {
			requiredSide = "Sell"
		}
		if existingSide == requiredSide {
			t.logger.Info("[Testnet] Bybit 已有同方向持仓，跳过",
				zap.String("symbol", opp.SymbolCode),
				zap.String("direction", opp.Direction),
				zap.String("bybit_side", existingSide),
				zap.String("bybit_size", existingPos.Size))
			return
		}
	}

	// 6. 检查本地数据库是否已有同交易对同方向的 open 记录
	for _, existingTrack := range openTracks {
		if existingTrack.SymbolID == opp.SymbolID && existingTrack.Direction == opp.Direction {
			t.logger.Info("[Testnet] 本地已有同方向持仓，跳过",
				zap.String("symbol", opp.SymbolCode),
				zap.String("direction", opp.Direction),
				zap.Int("existing_track_id", existingTrack.ID))
			return
		}
	}

	// 7. 获取入场价格和时间
	entryPrice, entryTime := GetEntryPriceAndTime(opp, t.klineRepo)
	if entryPrice <= 0 {
		t.logger.Warn("[Testnet] 无法获取入场价格，跳过",
			zap.String("symbol", opp.SymbolCode),
			zap.Int("opportunity_id", opp.ID))
		return
	}

	// 8. 设置杠杆
	leverage := testnetCfg.Leverage
	if leverage <= 0 {
		leverage = 2
	}
	if err := t.client.SetLeverage(opp.SymbolCode, leverage); err != nil {
		t.logger.Warn("[Testnet] 设置杠杆失败（继续下单）",
			zap.String("symbol", opp.SymbolCode),
			zap.Error(err))
	}

	// 9. 计算下单参数
	tradeAmount := testnetCfg.FixedTradeAmount
	if tradeAmount <= 0 {
		tradeAmount = 100
	}
	positionValue := float64(leverage) * tradeAmount // 杠杆后的仓位价值

	quantity := positionValue / entryPrice // 用杠杆后的仓位价值计算数量

		// 获取交易对信息，按 qtyStep 对齐数量
		var qtyStr string
		instrumentInfo, instrumentErr := t.client.GetInstrumentInfo(opp.SymbolCode)
		if instrumentErr != nil {
			t.logger.Warn("[Testnet] 获取交易对信息失败，使用默认精度",
				zap.String("symbol", opp.SymbolCode),
				zap.Error(instrumentErr))
		}
		if instrumentInfo != nil {
			qtyStepStr := instrumentInfo.LotSizeFilter.QtyStep
			if qtyStepStr == "" {
				qtyStepStr = instrumentInfo.QtyStep
			}
			qtyStep, _ := strconv.ParseFloat(qtyStepStr, 64)
			if qtyStep > 0 {
				qtyStr = FormatQty(quantity, qtyStep)
			} else {
				qtyStr = strconv.FormatFloat(quantity, 'f', 6, 64)
			}
		} else {
			qtyStr = strconv.FormatFloat(quantity, 'f', 6, 64)
		}

	// 确定方向
	var side string
	if opp.Direction == models.DirectionLong {
		side = "Buy"
	} else {
		side = "Sell"
	}

	// 计算 SL/TP
	stopLossPrice, takeProfitPrice := CalcSLTP(entryPrice, opp, t.config, t.klineRepo, t.logger)
	stopLossPrice, takeProfitPrice = ValidateRiskReward(entryPrice, opp.Direction, stopLossPrice, takeProfitPrice, t.config.MinRiskRewardRatio)

	slStr := strconv.FormatFloat(stopLossPrice, 'f', 6, 64)
	tpStr := strconv.FormatFloat(takeProfitPrice, 'f', 6, 64)

	// 10. 在 Bybit Testnet 下单
	orderResp, err := t.client.PlaceOrder(&PlaceOrderRequest{
		Symbol:     opp.SymbolCode,
		Side:       side,
		OrderType:  "Market",
		Qty:        qtyStr,
		StopLoss:   slStr,
		TakeProfit: tpStr,
	})
	if err != nil {
		t.logger.Error("[Testnet] Bybit 下单失败",
			zap.String("symbol", opp.SymbolCode),
			zap.String("side", side),
			zap.Error(err))
		return
	}

	// 10. 创建本地交易记录
	fees := positionValue * 0.0004 * 2 // 预估手续费（基于仓位价值）
	now := time.Now()
	oppID := opp.ID
	track := &models.TradeTrack{
		OpportunityID:         &oppID,
		SymbolID:              opp.SymbolID,
		Direction:             opp.Direction,
		EntryPrice:            &entryPrice,
		EntryTime:             &entryTime,
		Quantity:              &quantity,
		PositionValue:         &positionValue,
		StopLossPrice:         &stopLossPrice,
		StopLossPercent:       ptrFloat64(t.config.StopLossPercent),
		TakeProfitPrice:       &takeProfitPrice,
		TakeProfitPercent:     ptrFloat64(t.config.TakeProfitPercent),
		TrailingStopEnabled:   false, // testnet 使用交易所原生 SL/TP
		TrailingStopActive:    false,
		Status:                models.TrackStatusOpen,
		SubscriberCount:       1,
		Fees:                  fees,
		TradeSource:           models.TradeSourceTestnet,
		ExchangeOrderID:       orderResp.OrderID,
		CreatedAt:             now,
		UpdatedAt:             now,
	}

	if err := t.trackRepo.Create(track); err != nil {
		t.logger.Error("[Testnet] 创建交易记录失败",
			zap.String("symbol", opp.SymbolCode),
			zap.Error(err))
		return
	}

	// 从 Bybit 获取实际成交价，更新本地记录（用实际成交价替代本地计算的入场价）
	if orderResp.OrderID != "" {
		avgPrice, fillQty, fillFee, err := t.client.GetFilledOrderAvgPrice(opp.SymbolCode, orderResp.OrderID)
		if err == nil && avgPrice > 0 && fillQty > 0 {
			track.EntryPrice = &avgPrice
			track.Quantity = &fillQty
			track.Fees = fillFee
			if err := t.trackRepo.Update(track); err != nil {
				t.logger.Warn("[Testnet] 更新实际成交价失败", zap.Error(err))
			} else {
				t.logger.Info("[Testnet] 已用 Bybit 实际成交价更新记录",
					zap.String("symbol", opp.SymbolCode),
					zap.String("order_id", orderResp.OrderID),
					zap.Float64("avg_entry_price", avgPrice),
					zap.Float64("fill_qty", fillQty),
					zap.Float64("fill_fee", fillFee))
			}
		} else {
			t.logger.Warn("[Testnet] 获取 Bybit 成交明细失败",
				zap.String("symbol", opp.SymbolCode),
				zap.String("order_id", orderResp.OrderID),
				zap.Error(err))
		}
	}

	t.logger.Info("[Testnet] 开仓成功",
		zap.String("symbol", opp.SymbolCode),
		zap.String("direction", opp.Direction),
		zap.Int("score", opp.Score),
		zap.Int("opportunity_id", opp.ID),
		zap.Float64("entry_price", entryPrice),
		zap.String("order_id", orderResp.OrderID),
		zap.Float64("quantity", quantity),
		zap.Float64("stop_loss", stopLossPrice),
		zap.Float64("take_profit", takeProfitPrice))
}

// ClosePosition 通过 Bybit API 平仓
func (t *TestnetTrader) ClosePosition(track *models.TradeTrack, symbolCode string) error {
	if track.Quantity == nil {
		return fmt.Errorf("仓位数量为空")
	}

	var side string
	if track.Direction == models.DirectionLong {
		side = "Buy"
	} else {
		side = "Sell"
	}

	qtyStr := strconv.FormatFloat(*track.Quantity, 'f', 6, 64)

	if err := t.client.ClosePosition(symbolCode, side, qtyStr); err != nil {
		return fmt.Errorf("Bybit 平仓失败: %w", err)
	}

	return nil
}
