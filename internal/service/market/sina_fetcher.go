package market

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/smallfire/starfire/internal/config"
)

// SinaFetcher 新浪财经A股行情抓取器
// 备选数据源：当东方财富失败时降级使用
type SinaFetcher struct {
	client  *http.Client
	baseURL string
	config  config.MarketConfig
}

// NewSinaFetcher 创建新浪财经抓取器
func NewSinaFetcher(cfg config.MarketConfig) *SinaFetcher {
	return &SinaFetcher{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: "https://vip.stock.finance.sina.com.cn",
		config:  cfg,
	}
}

func (f *SinaFetcher) MarketCode() string {
	return "a_stock"
}

func (f *SinaFetcher) SupportedPeriods() []string {
	return []string{"daily", "weekly", "monthly"}
}

// FetchSymbols 获取A股股票列表（按成交额排序）
func (f *SinaFetcher) FetchSymbols() ([]SymbolInfo, error) {
	url := fmt.Sprintf(
		"%s/quotes_service/api/json_v2.php/Market_Center.getHQNodeData?page=1&num=500&sort=volume&asc=0&node=hs_a&symbol=&_s_r_a=page",
		f.baseURL,
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://finance.sina.com.cn")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 最多读1MB
	if err != nil {
		return nil, err
	}

	content := string(body)
	content = strings.TrimPrefix(content, "var a_s_hs_a=")
	content = strings.Trim(content, "; \n\t\r")

	if content == "" || content == "null" || content == "false" {
		return nil, fmt.Errorf("sina返回数据为空")
	}

	var rawResult [][]interface{}
	if err := json.Unmarshal([]byte(content), &rawResult); err != nil {
		return nil, fmt.Errorf("sina解析失败: %w", err)
	}

	var symbols []SymbolInfo
	for _, item := range rawResult {
		if len(item) < 10 {
			continue
		}

		symbolCode := toStr(item, 0)
		name := toStr(item, 1)
		if symbolCode == "" || name == "" {
			continue
		}

		// 过滤ST股票
		if strings.Contains(name, "ST") || strings.Contains(name, "st") {
			continue
		}

		// 解析成交额作为热度分数
		amount := toF64(item, 6)
		if amount <= 0 {
			continue
		}

		symbols = append(symbols, SymbolInfo{
			Code:     symbolCode,
			Name:     name,
			Type:     "stock",
			HotScore: amount,
		})
	}

	return symbols, nil
}

// FetchKlines 获取K线数据
func (f *SinaFetcher) FetchKlines(symbol, period string, limit int) ([]KlineData, error) {
	periodMap := map[string]string{
		"daily":   "d",
		"weekly":   "w",
		"monthly":  "m",
	}
	pt := periodMap[period]
	if pt == "" {
		pt = "d"
	}

	prefix := "sh"
	code := symbol
	if strings.HasPrefix(symbol, "sz") {
		prefix = "sz"
		code = strings.TrimPrefix(symbol, "sz")
	} else if strings.HasPrefix(symbol, "sh") {
		code = strings.TrimPrefix(symbol, "sh")
	}

	url := fmt.Sprintf(
		"%s/quotes_service/api/json_v2.php/CN_MarketData.getKLineData?symbol=%s%s&scale=%s&ma=no&datalen=%d",
		f.baseURL, prefix, code, pt, limit,
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Referer", "https://finance.sina.com.cn")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	content := strings.Trim(strings.Trim(string(body), "; \n\t\r"), ";")

	var rawData [][]interface{}
	if err := json.Unmarshal([]byte(content), &rawData); err != nil {
		return nil, fmt.Errorf("sina kline解析失败: %w", err)
	}

	var klines []KlineData
	for _, item := range rawData {
		if len(item) < 6 {
			continue
		}
		day := toStr(item, 0)
		openTime, err := time.ParseInLocation("2006-01-02", day, time.FixedZone("CST", 8*3600))
		if err != nil {
			continue
		}
		// 统一转为 UTC 存储
		openTime = openTime.UTC()

		klines = append(klines, KlineData{
			OpenTime:  openTime,
			CloseTime: openTime.Add(24 * time.Hour),
			Open:     toF64(item, 1),
			High:     toF64(item, 3),
			Low:      toF64(item, 4),
			Close:    toF64(item, 2),
			Volume:   toF64(item, 5),
		})
	}

	return klines, nil
}

// FetchKlinesByTimeRange 获取指定时间范围的K线
func (f *SinaFetcher) FetchKlinesByTimeRange(symbol, period string, startTime, endTime time.Time) ([]KlineData, error) {
	return f.FetchKlines(symbol, period, 500)
}

// FetchTicker 获取实时行情
func (f *SinaFetcher) FetchTicker(symbol string) (*Ticker, error) {
	prefix := "sh"
	code := symbol
	if strings.HasPrefix(symbol, "sz") {
		prefix = "sz"
		code = strings.TrimPrefix(symbol, "sz")
	} else if strings.HasPrefix(symbol, "sh") {
		code = strings.TrimPrefix(symbol, "sh")
	}

	url := fmt.Sprintf(
		"%s/quotes_service/api/json_v2.php/CN_MarketData.getStockQuoteInfo?symbol=%s%s",
		f.baseURL, prefix, code,
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Referer", "https://finance.sina.com.cn")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	content := strings.Trim(strings.Trim(string(body), "; \n\t\r"), ";")
	if content == "" {
		return nil, fmt.Errorf("sina无行情数据: %s", symbol)
	}

	var rawData []map[string]interface{}
	if err := json.Unmarshal([]byte(content), &rawData); err != nil {
		return nil, fmt.Errorf("sina行情解析失败: %w", err)
	}

	if len(rawData) == 0 {
		return nil, fmt.Errorf("sina无行情数据: %s", symbol)
	}

	data := rawData[0]
	price := toF64Map(data, "price")
	prevClose := toF64Map(data, "prev_close")
	high := toF64Map(data, "high")
	low := toF64Map(data, "low")
	vol := toF64Map(data, "volume")

	var changePct float64
	if prevClose > 0 {
		changePct = (price - prevClose) / prevClose * 100
	}

	return &Ticker{
		Symbol:    symbol,
		LastPrice: price,
		High24h:   high,
		Low24h:    low,
		Volume24h: vol,
		ChangePct: changePct,
		Timestamp: time.Now().Unix(),
	}, nil
}

// toStr 安全取string
func toStr(item []interface{}, idx int) string {
	if idx >= len(item) {
		return ""
	}
	if s, ok := item[idx].(string); ok {
		return s
	}
	return ""
}

// toF64 安全取float64
func toF64(item []interface{}, idx int) float64 {
	if idx >= len(item) {
		return 0
	}
	switch v := item[idx].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case string:
		f, _ := strconv.ParseFloat(v, 64)
		return f
	}
	return 0
}

// toF64Map 从map安全取float64
func toF64Map(m map[string]interface{}, key string) float64 {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	}
	return 0
}
