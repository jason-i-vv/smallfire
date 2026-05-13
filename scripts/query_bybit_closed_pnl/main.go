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

type closedPnlItem struct {
	Symbol      string `json:"symbol"`
	OrderID     string `json:"orderId"`
	Side        string `json:"side"`
	Qty         string `json:"qty"`
	EntryPrice  string `json:"entryPrice"`
	ExitPrice   string `json:"exitPrice"`
	ClosedPnl   string `json:"closedPnl"`
	Fee         string `json:"fee"`
	CreatedTime int64  `json:"createdTime"`
}

type apiResp struct {
	RetCode int    `json:"retCode"`
	RetMsg  string `json:"retMsg"`
	Result  struct {
		List           []closedPnlItem `json:"list"`
		NextPageCursor string          `json:"nextPageCursor"`
	}
}

func sign(pathWithQuery string) (string, int64) {
	ts := time.Now().UnixMilli()
	parts := strings.SplitN(pathWithQuery, "?", 2)
	paramStr := parts[1]
	payload := fmt.Sprintf("%d%s%s%s", ts, apiKey, recvWin, paramStr)
	mac := hmac.New(sha256.New, []byte(apiSecret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil)), ts
}

func callBybit(path string) ([]closedPnlItem, string, error) {
	sig, ts := sign(path)
	req, _ := http.NewRequest("GET", baseURL+path, nil)
	req.Header.Set("X-BAPI-API-KEY", apiKey)
	req.Header.Set("X-BAPI-TIMESTAMP", fmt.Sprintf("%d", ts))
	req.Header.Set("X-BAPI-SIGN", sig)
	req.Header.Set("X-BAPI-RECV-WINDOW", recvWin)

	httpResp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer httpResp.Body.Close()

	body, _ := io.ReadAll(httpResp.Body)
	var r apiResp
	json.Unmarshal(body, &r)
	if r.RetCode != 0 {
		return nil, "", fmt.Errorf("Bybit API error: %s", r.RetMsg)
	}
	return r.Result.List, r.Result.NextPageCursor, nil
}

func main() {
	symbols := os.Args[1:]
	if len(symbols) == 0 {
		fmt.Println("Usage: go run main.go SYMBOL [SYMBOL...]")
		return
	}

	for _, sym := range symbols {
		fmt.Printf("\n=== %s ===\n", sym)

		list, cursor, err := callBybit(fmt.Sprintf("/v5/position/closed-pnl?category=linear&symbol=%s&limit=50", sym))
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}
		fmt.Printf("Page 1: %d records\n", len(list))

		for _, p := range list {
			ts := time.UnixMilli(p.CreatedTime).Format("2006-01-02 15:04:05")
			fmt.Printf("  [%s] orderID=%s side=%s qty=%s entry=%s exit=%s pnl=%s fee=%s\n",
				ts, p.OrderID, p.Side, p.Qty, p.EntryPrice, p.ExitPrice, p.ClosedPnl, p.Fee)
		}

		page := 2
		for cursor != "" && page <= 3 {
			time.Sleep(200 * time.Millisecond)
			list2, cursor2, err := callBybit(fmt.Sprintf("/v5/position/closed-pnl?category=linear&symbol=%s&limit=50&cursor=%s", sym, cursor))
			if err != nil {
				fmt.Printf("Page %d error: %v\n", page, err)
				break
			}
			fmt.Printf("Page %d: %d records\n", page, len(list2))
			for _, p := range list2 {
				ts := time.UnixMilli(p.CreatedTime).Format("2006-01-02 15:04:05")
				fmt.Printf("  [%s] orderID=%s side=%s qty=%s entry=%s exit=%s pnl=%s fee=%s\n",
					ts, p.OrderID, p.Side, p.Qty, p.EntryPrice, p.ExitPrice, p.ClosedPnl, p.Fee)
			}
			cursor = cursor2
			page++
		}
	}
}
