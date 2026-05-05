package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	apiKey    = "DatS4UH1Leyg9qG2Kc"
	apiSecret = "UlUKA1qDeVRa3XYAuzDbGVyWqZZICWvvmY6y"
	baseURL   = "https://api-demo.bybit.com"
	recvWin   = "5000"
)

type closedPnl struct {
	Symbol     string  `json:"symbol"`
	OrderID    string  `json:"orderId"`
	Side       string  `json:"side"`
	Qty        float64 `json:"qty"`
	EntryPrice float64 `json:"entryPrice"`
	ExitPrice  float64 `json:"exitPrice"`
	ClosedPnl  float64 `json:"closedPnl"`
	Fee        float64 `json:"fee"`
	CreatedTime int64  `json:"createdTime"`
}

func sign(path string) (string, int64) {
	ts := time.Now().UnixMilli()
	paramStr := strings.SplitN(path, "?", 2)[1]
	payload := fmt.Sprintf("%d%s%s%s", ts, apiKey, recvWin, paramStr)
	mac := hmac.New(sha256.New, []byte(apiSecret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil)), ts
}

func callBybit(path string) ([]closedPnl, error) {
	sig, ts := sign(path)
	req, _ := http.NewRequest("GET", baseURL+path, nil)
	req.Header.Set("X-BAPI-API-KEY", apiKey)
	req.Header.Set("X-BAPI-TIMESTAMP", fmt.Sprintf("%d", ts))
	req.Header.Set("X-BAPI-SIGN", sig)
	req.Header.Set("X-BAPI-RECV-WINDOW", recvWin)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var r struct {
		Result struct {
			List []closedPnl `json:"list"`
		} `json:"result"`
	}
	json.Unmarshal(body, &r)
	return r.Result.List, nil
}

type record struct {
	ID             int
	SymbolCode     string
	OrderID        string
	LocalEntry     float64
	LocalExit      float64
	LocalPnL       float64
	Direction      string
}

func main() {
	// 1. 从数据库读取所有 testnet closed 且有 exchange_order_id 的 unknown 记录
	// 2. 按 symbol 批量查 bybit closed-pnl
	// 3. 用 order_id 匹配
	// 4. 输出 UPDATE SQL 语句

	// 读取记录（通过环境变量传入）
	data := os.Getenv("RECORDS")
	if data == "" {
		fmt.Println("No records provided")
		return
	}

	var records []record
	json.Unmarshal([]byte(data), &records)

	fmt.Printf("共 %d 条记录待修复\n", len(records))

	// 按 symbol 分组
	symMap := make(map[string][]int)
	for i, r := range records {
		symMap[r.SymbolCode] = append(symMap[r.SymbolCode], i)
	}

	var updates []string

	for symbol := range symMap {
		pnls, err := callBybit(fmt.Sprintf("/v5/position/closed-pnl?category=linear&symbol=%s&limit=50", symbol))
		if err != nil {
			fmt.Printf("  [%s] 查询失败: %v\n", symbol, err)
			continue
		}
		fmt.Printf("  [%s] 查到 %d 条 bybit 记录\n", symbol, len(pnls))

		// 构建 orderId -> bybit pnl 的映射
		pnlMap := make(map[string]closedPnl)
		for _, p := range pnls {
			pnlMap[p.OrderID] = p
		}

		// 匹配
		for _, idx := range symMap[symbol] {
			r := records[idx]
			bp, ok := pnlMap[r.OrderID]
			if !ok {
				fmt.Printf("  [%s] order_id %s 未找到\n", symbol, r.OrderID)
				continue
			}

			// 计算实际 PnL（bybit 的 closedPnl 已扣费）
			direction := r.Direction
			var pnl float64
			if direction == "long" {
				pnl = (bp.ExitPrice - bp.EntryPrice) * bp.Qty
			} else {
				pnl = (bp.EntryPrice - bp.ExitPrice) * bp.Qty
			}
			// bybit 的 closedPnl 已包含 fees（已扣费）
			// 使用 bybit 返回的真实 closedPnl
			pnl = bp.ClosedPnl

			exitReason := "stop_loss"
			if pnl >= 0 {
				exitReason = "take_profit"
			}

			sql := fmt.Sprintf("UPDATE trade_tracks SET entry_price=%.8f, exit_price=%.8f, pnl=%.8f, fees=%.8f, quantity=%.8f, exit_reason='%s', updated_at=NOW() WHERE id=%d; -- %s",
				bp.EntryPrice, bp.ExitPrice, pnl, bp.Fee, bp.Qty, exitReason, r.ID, symbol)
			updates = append(updates, sql)
			fmt.Printf("  [%d/%s] matched: entry=%.6f exit=%.6f pnl=%.4f\n", r.ID, symbol, bp.EntryPrice, bp.ExitPrice, pnl)
		}
	}

	fmt.Printf("\n=== 共生成 %d 条 UPDATE ===\n", len(updates))
	for _, u := range updates {
		fmt.Println(u)
	}
}