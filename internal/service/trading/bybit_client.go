package trading

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

// BybitTradingClient Bybit 交易所交易客户端
// 支持 Bybit V5 API 的下单、查仓、平仓等操作
type BybitTradingClient struct {
	client     *http.Client
	baseURL    string
	apiKey     string
	apiSecret  string
	recvWindow string
	logger     *zap.Logger
}

// NewBybitTradingClient 创建 Bybit 交易客户端
func NewBybitTradingClient(baseURL, apiKey, apiSecret, recvWindow string, logger *zap.Logger) *BybitTradingClient {
	if recvWindow == "" {
		recvWindow = "5000"
	}
	return &BybitTradingClient{
		client:     &http.Client{Timeout: 30 * time.Second},
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		apiKey:     apiKey,
		apiSecret:  apiSecret,
		recvWindow: recvWindow,
		logger:     logger,
	}
}

// PlaceOrderRequest 下单请求
type PlaceOrderRequest struct {
	Symbol      string // 交易对，如 "BTCUSDT"
	Side        string // "Buy" 或 "Sell"
	OrderType   string // "Market"
	Qty         string // 数量
	StopLoss    string // 止损价
	TakeProfit  string // 止盈价
	SLTriggerBy string // 止损触发类型: "LastPrice"
	TPTriggerBy string // 止盈触发类型: "LastPrice"
}

// PlaceOrderResponse 下单响应
type PlaceOrderResponse struct {
	OrderID    string `json:"orderId"`
	Account    string `json:"acctId"`
	Symbol     string `json:"symbol"`
	CreateTime string `json:"createdTime"`
}

// PositionInfo 仓位信息
type PositionInfo struct {
	Symbol         string  `json:"symbol"`
	Side           string  `json:"side"`
	Size           string  `json:"size"`
	EntryPrice     string  `json:"entryPrice"`
	UnrealizedPnL  string  `json:"unrealisedPnl"`
	MarkPrice      string  `json:"markPrice"`
	LiqPrice       string  `json:"liqPrice"`
	StopLoss       string  `json:"stopLoss"`
	TakeProfit     string  `json:"takeProfit"`
	CreatedTime    string  `json:"createdTime"`
	UpdatedTime    string  `json:"updatedTime"`
}

// OrderInfo 订单信息
type OrderInfo struct {
	OrderID     string `json:"orderId"`
	Symbol      string `json:"symbol"`
	Side        string `json:"side"`
	OrderType   string `json:"orderType"`
	Price       string `json:"price"`
	Qty         string `json:"qty"`
	StopOrderType string `json:"stopOrderType"`
	TriggerPrice  string `json:"triggerPrice"`
	OrderStatus   string `json:"orderStatus"`
	CumExecQty    string `json:"cumExecQty"`
	CumExecFee    string `json:"cumExecFee"`
	CumExecAmt    string `json:"cumExecValue"`
	AvgPrice      string `json:"avgPrice"`
	CreatedTime   string `json:"createdTime"`
	UpdatedTime   string `json:"updatedTime"`
}

// bybitAPIResponse Bybit API 通用响应
type bybitAPIResponse struct {
	RetCode int             `json:"retCode"`
	RetMsg  string          `json:"retMsg"`
	Result  json.RawMessage `json:"result"`
}

// sign 生成 HMAC-SHA256 签名
func (c *BybitTradingClient) sign(payload string) string {
	mac := hmac.New(sha256.New, []byte(c.apiSecret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

// buildAuthHeaders 构建认证请求头
func (c *BybitTradingClient) buildAuthHeaders(timestamp int64, sign string) http.Header {
	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	headers.Set("X-BAPI-API-KEY", c.apiKey)
	headers.Set("X-BAPI-TIMESTAMP", fmt.Sprintf("%d", timestamp))
	headers.Set("X-BAPI-SIGN", sign)
	headers.Set("X-BAPI-RECV-WINDOW", c.recvWindow)
	return headers
}

// authRequest 发送带认证的请求
func (c *BybitTradingClient) authRequest(method, path string, body interface{}) (*bybitAPIResponse, error) {
	timestamp := time.Now().UnixMilli()

	var bodyBytes []byte
	var bodyStr string
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("序列化请求体失败: %w", err)
		}
		bodyStr = string(bodyBytes)
	}

	// 签名: timestamp + apiKey + recvWindow + paramStr
	// GET 请求用 query string，POST 请求用 body JSON
	paramStr := bodyStr
	if method == "GET" {
		if idx := strings.Index(path, "?"); idx >= 0 {
			paramStr = path[idx+1:]
		}
	}
	payload := fmt.Sprintf("%d%s%s%s", timestamp, c.apiKey, c.recvWindow, paramStr)
	sign := c.sign(payload)

	url := c.baseURL + path
	var req *http.Request
	var err error

	if body != nil {
		req, err = http.NewRequest(method, url, strings.NewReader(bodyStr))
	} else {
		req, err = http.NewRequest(method, url, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header = c.buildAuthHeaders(timestamp, sign)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var apiResp bybitAPIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %s", string(respBody))
	}

	if apiResp.RetCode != 0 {
		return &apiResp, fmt.Errorf("Bybit API 错误: code=%d msg=%s", apiResp.RetCode, apiResp.RetMsg)
	}

	return &apiResp, nil
}

// PlaceOrder 下单
func (c *BybitTradingClient) PlaceOrder(req *PlaceOrderRequest) (*PlaceOrderResponse, error) {
	body := map[string]string{
		"category":  "linear",
		"symbol":    req.Symbol,
		"side":      req.Side,
		"orderType": req.OrderType,
		"qty":       req.Qty,
	}

	if req.StopLoss != "" {
		body["stopLoss"] = req.StopLoss
		body["slTriggerBy"] = "LastPrice"
		if req.SLTriggerBy != "" {
			body["slTriggerBy"] = req.SLTriggerBy
		}
	}
	if req.TakeProfit != "" {
		body["takeProfit"] = req.TakeProfit
		body["tpTriggerBy"] = "LastPrice"
		if req.TPTriggerBy != "" {
			body["tpTriggerBy"] = req.TPTriggerBy
		}
	}

	c.logger.Info("Bybit 下单请求",
		zap.String("symbol", req.Symbol),
		zap.String("side", req.Side),
		zap.String("qty", req.Qty),
		zap.String("stop_loss", req.StopLoss),
		zap.String("take_profit", req.TakeProfit))

	apiResp, err := c.authRequest("POST", "/v5/order/create", body)
	if err != nil {
		return nil, fmt.Errorf("下单失败: %w", err)
	}

	var result struct {
		OrderID    string `json:"orderId"`
		Account    string `json:"acctId"`
		Symbol     string `json:"symbol"`
		CreateTime string `json:"createdTime"`
	}
	if err := json.Unmarshal(apiResp.Result, &result); err != nil {
		return nil, fmt.Errorf("解析下单响应失败: %w", err)
	}

	c.logger.Info("Bybit 下单成功",
		zap.String("order_id", result.OrderID),
		zap.String("symbol", result.Symbol))

	return &PlaceOrderResponse{
		OrderID:    result.OrderID,
		Account:    result.Account,
		Symbol:     result.Symbol,
		CreateTime: result.CreateTime,
	}, nil
}

// SetLeverage 设置杠杆倍数
func (c *BybitTradingClient) SetLeverage(symbol string, leverage int) error {
	body := map[string]string{
		"category":     "linear",
		"symbol":       symbol,
		"buyLeverage":  strconv.Itoa(leverage),
		"sellLeverage": strconv.Itoa(leverage),
	}

	c.logger.Info("Bybit 设置杠杆",
		zap.String("symbol", symbol),
		zap.Int("leverage", leverage))

	_, err := c.authRequest("POST", "/v5/position/set-leverage", body)
	if err != nil {
		// 110043 = leverage not modified，视为成功
		if strings.Contains(err.Error(), "code=110043") {
			c.logger.Info("杠杆未变更（已为目标值）", zap.String("symbol", symbol), zap.Int("leverage", leverage))
			return nil
		}
		return fmt.Errorf("设置杠杆失败: %w", err)
	}
	return nil
}

// QueryPosition 查询仓位
func (c *BybitTradingClient) QueryPosition(symbol string) (*PositionInfo, error) {
	path := fmt.Sprintf("/v5/position/list?category=linear&symbol=%s", symbol)

	apiResp, err := c.authRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("查询仓位失败: %w", err)
	}

	var result struct {
		List []PositionInfo `json:"list"`
	}
	if err := json.Unmarshal(apiResp.Result, &result); err != nil {
		return nil, fmt.Errorf("解析仓位响应失败: %w", err)
	}

	if len(result.List) == 0 {
		return nil, nil // 无仓位
	}

	return &result.List[0], nil
}

// QueryAllPositions 批量查询所有持仓（不分页，默认100条）
func (c *BybitTradingClient) QueryAllPositions() ([]PositionInfo, error) {
	path := "/v5/position/list?category=linear&settleCoin=USDT&limit=100"

	apiResp, err := c.authRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("批量查询持仓失败: %w", err)
	}

	var result struct {
		List       []PositionInfo `json:"list"`
		NextPageCursor string     `json:"nextPageCursor"`
	}
	if err := json.Unmarshal(apiResp.Result, &result); err != nil {
		return nil, fmt.Errorf("解析持仓响应失败: %w", err)
	}

	// 过滤出 size > 0 的持仓
	openPositions := make([]PositionInfo, 0)
	for _, pos := range result.List {
		if pos.Size != "0" && pos.Size != "" {
			openPositions = append(openPositions, pos)
		}
	}

	return openPositions, nil
}

// ClosePosition 平仓
func (c *BybitTradingClient) ClosePosition(symbol string, side string, qty string) error {
	// Bybit 平仓：用相反方向下一个 Market 单
	var closeSide string
	if side == "Buy" {
		closeSide = "Sell"
	} else {
		closeSide = "Buy"
	}

	body := map[string]string{
		"category":  "linear",
		"symbol":    symbol,
		"side":      closeSide,
		"orderType": "Market",
		"qty":       qty,
		"reduceOnly": "true",
	}

	c.logger.Info("Bybit 平仓请求",
		zap.String("symbol", symbol),
		zap.String("side", closeSide),
		zap.String("qty", qty))

	_, err := c.authRequest("POST", "/v5/order/create", body)
	if err != nil {
		return fmt.Errorf("平仓失败: %w", err)
	}

	return nil
}

// GetOrderHistory 查询订单历史
func (c *BybitTradingClient) GetOrderHistory(symbol, orderID string) (*OrderInfo, error) {
	path := fmt.Sprintf("/v5/order/history?category=linear&symbol=%s", symbol)
	if orderID != "" {
		path += fmt.Sprintf("&orderId=%s", orderID)
	}

	apiResp, err := c.authRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("查询订单历史失败: %w", err)
	}

	var result struct {
		List []OrderInfo `json:"list"`
	}
	if err := json.Unmarshal(apiResp.Result, &result); err != nil {
		return nil, fmt.Errorf("解析订单历史响应失败: %w", err)
	}

	if len(result.List) == 0 {
		return nil, nil
	}

	return &result.List[0], nil
}

// ClosedPnlInfo 已平仓盈亏信息
type ClosedPnlInfo struct {
	Symbol       string `json:"symbol"`
	Side         string `json:"side"`
	Qty          string `json:"qty"`
	EntryPrice   string `json:"entryPrice"`
	ExitPrice    string `json:"exitPrice"`
	ClosedPnl    string `json:"closedPnl"`
	ClosedSize   string `json:"closedSize"`
	Fee          string `json:"fee"`
	OccuringTime int64  `json:"occuringTime"` // Unix timestamp (毫秒)
}

// GetClosedPnlBySymbol 获取已平仓盈亏记录
// symbol 为空时查询所有币对的最近记录，symbol 格式如 "TRXUSDT"，limit 默认 20
func (c *BybitTradingClient) GetClosedPnlBySymbol(symbol string, limit int) ([]ClosedPnlInfo, error) {
	if limit <= 0 {
		limit = 20
	}
	path := "/v5/position/closed-pnl?category=linear&limit=" + strconv.Itoa(limit)
	if symbol != "" {
		path += "&symbol=" + symbol
	}

	apiResp, err := c.authRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("查询已平仓盈亏失败: %w", err)
	}

	var result struct {
		List []ClosedPnlInfo `json:"list"`
	}
	if err := json.Unmarshal(apiResp.Result, &result); err != nil {
		return nil, fmt.Errorf("解析已平仓盈亏响应失败: %w", err)
	}

	return result.List, nil
}

// GetTickerPrice 获取最新价格（公开 API，不需要认证）
func (c *BybitTradingClient) GetTickerPrice(symbol string) (float64, error) {
	url := fmt.Sprintf("%s/v5/market/tickers?category=linear&symbol=%s", c.baseURL, symbol)

	resp, err := c.client.Get(url)
	if err != nil {
		return 0, fmt.Errorf("获取行情失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("读取行情响应失败: %w", err)
	}

	var result struct {
		RetCode int `json:"retCode"`
		Data    struct {
			List []struct {
				LastPrice string `json:"lastPrice"`
			} `json:"list"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("解析行情响应失败: %w", err)
	}

	if result.RetCode != 0 || len(result.Data.List) == 0 {
		return 0, fmt.Errorf("获取行情数据为空")
	}

	price, err := strconv.ParseFloat(result.Data.List[0].LastPrice, 64)
	if err != nil {
		return 0, fmt.Errorf("解析价格失败: %w", err)
	}

	return price, nil
}

// ExecDetail 单条执行记录
type ExecDetail struct {
	ExecID       string  `json:"execId"`
	Symbol       string  `json:"symbol"`
	Side         string  `json:"side"`
	OrderID      string  `json:"orderId"`
	ExecPrice    float64 `json:"execPrice"`
	ExecQty      float64 `json:"execQty"`
	ExecFee      float64 `json:"execFee"`
	FeeRate      float64 `json:"feeRate"`
	ExecTime     int64   `json:"execTime"` // Unix ms
}

// GetExecutions 获取订单的全部成交明细
func (c *BybitTradingClient) GetExecutions(symbol, orderID string) ([]ExecDetail, error) {
	path := fmt.Sprintf("/v5/execution/list?category=linear&symbol=%s&orderId=%s", symbol, orderID)
	apiResp, err := c.authRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("查询成交明细失败: %w", err)
	}

	var result struct {
		List []ExecDetail `json:"list"`
	}
	if err := json.Unmarshal(apiResp.Result, &result); err != nil {
		return nil, fmt.Errorf("解析成交明细失败: %w", err)
	}
	return result.List, nil
}

// GetFilledOrderAvgPrice 获取订单的平均成交价
// 返回 (avgPrice, totalQty, totalFee, error)
func (c *BybitTradingClient) GetFilledOrderAvgPrice(symbol, orderID string) (float64, float64, float64, error) {
	execs, err := c.GetExecutions(symbol, orderID)
	if err != nil || len(execs) == 0 {
		return 0, 0, 0, err
	}

	var totalValue, totalQty, totalFee float64
	for _, e := range execs {
		totalValue += e.ExecPrice * e.ExecQty
		totalQty += e.ExecQty
		totalFee += e.ExecFee
	}
	if totalQty == 0 {
		return 0, 0, 0, fmt.Errorf("无成交数量")
	}
	avgPrice := totalValue / totalQty
	return avgPrice, totalQty, totalFee, nil
}

// InstrumentInfo 交易对信息
type InstrumentInfo struct {
	QtyStep      string `json:"qtyStep"`
	MinOrderQty  string `json:"minOrderQty"`
	LotSizeFilter struct {
		QtyStep     string `json:"qtyStep"`
		MinOrderQty string `json:"minOrderQty"`
	} `json:"lotSizeFilter"`
}

// GetInstrumentInfo 获取交易对信息（公开 API，不需要认证）
func (c *BybitTradingClient) GetInstrumentInfo(symbol string) (*InstrumentInfo, error) {
	url := fmt.Sprintf("%s/v5/market/instruments-info?category=linear&symbol=%s", c.baseURL, symbol)

	resp, err := c.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("获取交易对信息失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取交易对信息失败: %w", err)
	}

	var result struct {
		RetCode int `json:"retCode"`
		Data    struct {
			List []InstrumentInfo `json:"list"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析交易对信息失败: %w", err)
	}

	if result.RetCode != 0 || len(result.Data.List) == 0 {
		return nil, fmt.Errorf("获取交易对信息为空")
	}

	return &result.Data.List[0], nil
}

// FormatQty 按 qtyStep 对齐数量精度
func FormatQty(qty float64, qtyStep float64) string {
	if qtyStep <= 0 {
		qtyStep = 1
	}
	// 计算步长的小数位数
	stepStr := strconv.FormatFloat(qtyStep, 'f', -1, 64)
	decimalPlaces := 0
	if idx := strings.Index(stepStr, "."); idx >= 0 {
		decimalPlaces = len(stepStr) - idx - 1
	}
	// 按 qtyStep 取整
	rounded := float64(int(qty/qtyStep)) * qtyStep
	return strconv.FormatFloat(rounded, 'f', decimalPlaces, 64)
}
