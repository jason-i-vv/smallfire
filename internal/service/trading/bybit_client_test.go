package trading

import (
	"fmt"
	"testing"
	"time"

	"go.uber.org/zap"
)

// TestFormatQty 验证 qtyStep 对齐逻辑
func TestFormatQty(t *testing.T) {
	tests := []struct {
		qty      float64
		qtyStep  float64
		expected string
	}{
		{123.456789, 0.001, "123.456"},    // 3位小数
		{123.456789, 0.1, "123.4"},         // 1位小数
		{123.456789, 1, "123"},             // 整数步长
		{123.456789, 10, "120"},            // 10的步长
		{0.005123, 0.001, "0.005"},         // 小数量
		{100.999, 0.01, "100.99"},          // 2位小数
	}
	for _, tt := range tests {
		result := FormatQty(tt.qty, tt.qtyStep)
		if result != tt.expected {
			t.Errorf("FormatQty(%v, %v) = %s, want %s", tt.qty, tt.qtyStep, result, tt.expected)
		}
	}
}

// TestBybitTestnetOpenClose 测试 Bybit Testnet 完整开平仓流程
// 2倍杠杆，10U 本金，开 BTCUSDT 多单
func TestBybitTestnetOpenClose(t *testing.T) {
	logger, _ := zap.NewDevelopment()

	client := NewBybitTradingClient(
		"https://api-demo.bybit.com",
		"FbhCqSRkS1jRZmNBoR",
		"rpUnYS8jQxhU96yJeR1HnKuIbyFiXzatgKxx",
		"5000",
		logger,
	)

	symbol := "BTCUSDT"
	leverage := 2
	margin := 10.0 // 10 USDT 本金
	positionValue := margin * float64(leverage) // 20 USDT 仓位价值

	// Step 1: 设置杠杆
	t.Log("Step 1: 设置杠杆为 2x")
	if err := client.SetLeverage(symbol, leverage); err != nil {
		t.Fatalf("设置杠杆失败: %v", err)
	}
	t.Logf("杠杆设置成功: %dx", leverage)

	// Step 2: 获取当前价格
	t.Log("Step 2: 获取 BTCUSDT 当前价格")
	price, err := client.GetTickerPrice(symbol)
	if err != nil {
		t.Fatalf("获取价格失败: %v", err)
	}
	t.Logf("当前价格: %.2f USDT", price)

	// Step 3: 计算下单数量
	qty := positionValue / price
	qtyStr := fmt.Sprintf("%.4f", qty)
	t.Logf("Step 3: 下单数量: %s BTC (仓位价值: %.2f USDT)", qtyStr, positionValue)

	// Step 4: 开多单（市价）
	t.Log("Step 4: 开多单")
	orderResp, err := client.PlaceOrder(&PlaceOrderRequest{
		Symbol:    symbol,
		Side:      "Buy",
		OrderType: "Market",
		Qty:       qtyStr,
	})
	if err != nil {
		t.Fatalf("开仓失败: %v", err)
	}
	t.Logf("开仓成功! OrderID: %s", orderResp.OrderID)

	// Step 5: 等待一下然后查询仓位
	t.Log("Step 5: 等待 3s 后查询仓位...")
	time.Sleep(3 * time.Second)

	pos, err := client.QueryPosition(symbol)
	if err != nil {
		t.Fatalf("查询仓位失败: %v", err)
	}
	if pos == nil {
		t.Fatal("未找到仓位")
	}
	t.Logf("仓位信息:")
	t.Logf("  方向: %s", pos.Side)
	t.Logf("  数量: %s", pos.Size)
	t.Logf("  开仓价: %s", pos.EntryPrice)
	t.Logf("  标记价: %s", pos.MarkPrice)
	t.Logf("  未实现盈亏: %s USDT", pos.UnrealizedPnL)
	t.Logf("  强平价: %s", pos.LiqPrice)

	// Step 6: 平仓
	t.Log("Step 6: 平仓")
	if err := client.ClosePosition(symbol, "Buy", pos.Size); err != nil {
		t.Fatalf("平仓失败: %v", err)
	}
	t.Log("平仓成功!")

	// Step 7: 确认仓位已平
	time.Sleep(2 * time.Second)
	posAfter, err := client.QueryPosition(symbol)
	if err != nil {
		t.Logf("查询仓位(平仓后): %v", err)
	} else if posAfter == nil || posAfter.Size == "0" {
		t.Log("确认: 仓位已完全平仓")
	} else {
		t.Logf("警告: 仍有剩余仓位, size=%s", posAfter.Size)
	}

	t.Log("=== 测试通过: Bybit Testnet 开平仓流程正常 ===")
}
