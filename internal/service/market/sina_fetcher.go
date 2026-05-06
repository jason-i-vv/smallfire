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

	var rawResult []map[string]interface{}
	if err := json.Unmarshal([]byte(content), &rawResult); err != nil {
		return nil, fmt.Errorf("sina解析失败: %w", err)
	}

	var symbols []SymbolInfo
	for _, item := range rawResult {
		symbolCode, _ := item["symbol"].(string)
		name, _ := item["name"].(string)
		if symbolCode == "" || name == "" {
			continue
		}

		// 过滤ST股票
		if strings.Contains(name, "ST") || strings.Contains(name, "st") {
			continue
		}

		// 解析成交额作为热度分数
		amount := toF64Map(item, "amount")
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

// sinaKlineItem 新浪K线API返回的单条数据结构
type sinaKlineItem struct {
	Day    string `json:"day"`
	Open   string `json:"open"`
	High   string `json:"high"`
	Low    string `json:"low"`
	Close  string `json:"close"`
	Volume string `json:"volume"`
}

// FetchKlines 获取K线数据
func (f *SinaFetcher) FetchKlines(symbol, period string, limit int) ([]KlineData, error) {
	// scale 参数：240=日K, 1200=周K, 7200=月K（分钟数）
	periodMap := map[string]string{
		"daily":   "240",
		"weekly":  "1200",
		"monthly": "7200",
	}
	pt := periodMap[period]
	if pt == "" {
		pt = "240"
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
	if content == "" || content == "null" {
		return nil, fmt.Errorf("sina kline返回数据为空")
	}

	// 检查是否返回错误
	if strings.Contains(content, "__ERROR") {
		return nil, fmt.Errorf("sina kline API错误: %s", content)
	}

	var items []sinaKlineItem
	if err := json.Unmarshal([]byte(content), &items); err != nil {
		return nil, fmt.Errorf("sina kline解析失败: %w", err)
	}

	var klines []KlineData
	for _, item := range items {
		openTime, err := time.ParseInLocation("2006-01-02", item.Day, time.FixedZone("CST", 8*3600))
		if err != nil {
			continue
		}
		// 统一转为 UTC 存储
		openTime = openTime.UTC()

		klines = append(klines, KlineData{
			OpenTime:  openTime,
			CloseTime: openTime.Add(24 * time.Hour),
			Open:      parseFloatStr(item.Open),
			High:      parseFloatStr(item.High),
			Low:       parseFloatStr(item.Low),
			Close:     parseFloatStr(item.Close),
			Volume:    parseFloatStr(item.Volume),
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

// FetchAStockIndices 获取A股大盘指数（Sina不支持，返回空）
func (f *SinaFetcher) FetchAStockIndices() ([]AStockMarketIndex, error) {
	return nil, fmt.Errorf("sina fetcher 不支持获取A股指数")
}

// FetchSectorList 获取板块涨跌榜（Sina不支持）
func (f *SinaFetcher) FetchSectorList(sortField string, ascending bool, limit int) ([]SectorData, error) {
	return nil, fmt.Errorf("sina fetcher 不支持获取板块数据")
}

// FetchLimitCount 获取涨跌停统计
// 通过遍历新浪涨跌幅排行榜，统计涨停（涨幅>=9.9%）和跌停（跌幅<=-9.9%）的股票数量
func (f *SinaFetcher) FetchLimitCount() (*LimitCount, error) {
	limitThreshold := 9.9

	// 串行获取涨停和跌停数量，避免并发导致请求过快被限流
	upCount, err := f.countLimitUpStocks(limitThreshold)
	if err != nil {
		return nil, err
	}

	downCount, err := f.countLimitDownStocks(limitThreshold)
	if err != nil {
		return nil, err
	}

	return &LimitCount{
		UpCount:   upCount,
		DownCount: downCount,
	}, nil
}

// countLimitUpStocks 统计涨停股票数量（涨幅 >= threshold）
func (f *SinaFetcher) countLimitUpStocks(threshold float64) (int, error) {
	const pageSize = 100
	var totalCount int

	// 按涨幅降序排列 (asc=0)，前面是涨停股
	// 数据按涨幅降序排列，一旦出现涨幅 < threshold，后面的都不是涨停
	for page := 1; page <= 50; page++ {
		url := fmt.Sprintf(
			"%s/quotes_service/api/json_v2.php/Market_Center.getHQNodeData?page=%d&num=%d&sort=changepercent&asc=0&node=hs_a&symbol=&_s_r_a=page",
			f.baseURL, page, pageSize,
		)

		count, reachedEnd, err := f.fetchAndCountLimitStocks(url, threshold, true)
		if err != nil {
			return 0, err
		}
		totalCount += count

		// 如果到达数据末尾（返回数量少于pageSize）或者这一页末尾的股票已低于阈值，则停止
		if reachedEnd || count < pageSize {
			break
		}
	}

	return totalCount, nil
}

// countLimitDownStocks 统计跌停股票数量（跌幅 <= -threshold）
func (f *SinaFetcher) countLimitDownStocks(threshold float64) (int, error) {
	const pageSize = 100
	var totalCount int

	// 按涨幅升序排列 (asc=1)，前面是跌停股
	for page := 1; page <= 50; page++ {
		url := fmt.Sprintf(
			"%s/quotes_service/api/json_v2.php/Market_Center.getHQNodeData?page=%d&num=%d&sort=changepercent&asc=1&node=hs_a&symbol=&_s_r_a=page",
			f.baseURL, page, pageSize,
		)

		count, reachedEnd, err := f.fetchAndCountLimitStocks(url, -threshold, false)
		if err != nil {
			return 0, err
		}
		totalCount += count

		if reachedEnd || count < pageSize {
			break
		}
	}

	return totalCount, nil
}

// fetchAndCountLimitStocks 获取一页数据并统计涨跌停数量
// threshold: 涨停为正数，跌停为负数
// isLimitUp: true=统计涨停，false=统计跌停
// 返回: 统计数量, 是否到达数据末尾, 错误
func (f *SinaFetcher) fetchAndCountLimitStocks(url string, threshold float64, isLimitUp bool) (int, bool, error) {
	const pageSize = 100 // Sina API 每页返回的最大数量

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, true, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://finance.sina.com.cn")

	resp, err := f.client.Do(req)
	if err != nil {
		return 0, true, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return 0, true, err
	}

	content := string(body)
	content = strings.TrimPrefix(content, "var a_s_hs_a=")
	content = strings.Trim(content, "; \n\t\r")

	if content == "" || content == "null" || content == "false" {
		return 0, true, nil
	}

	// Sina API 返回的是 []map[string]interface{} 格式
	var rawResult []map[string]interface{}
	if err := json.Unmarshal([]byte(content), &rawResult); err != nil {
		return 0, true, fmt.Errorf("sina解析失败: %w", err)
	}

	if len(rawResult) == 0 {
		return 0, true, nil
	}

	var count int
	for _, item := range rawResult {
		changePercent := toF64Map(item, "changepercent")

		if isLimitUp {
			// 涨停：涨幅 >= threshold
			if changePercent >= threshold {
				count++
			}
		} else {
			// 跌停：跌幅 <= threshold (threshold是负数)
			if changePercent <= threshold {
				count++
			}
		}
	}

	// 如果返回的数量少于pageSize，说明已经是最后一页
	reachedEnd := len(rawResult) < pageSize
	return count, reachedEnd, nil
}

// FetchIndexKlines 获取指数K线数据（使用东方财富API）
// indexCode: "sh000001" (上证), "sz399001" (深证), "sz399006" (创业板)
// period: "daily" (日K), "weekly" (周K), "monthly" (月K)
func (f *SinaFetcher) FetchIndexKlines(indexCode string, period string, limit int) ([]KlineData, error) {
	// 东方财富 Kline API
	kltMap := map[string]int{
		"daily":   101,
		"weekly":  102,
		"monthly": 103,
	}
	klt := kltMap[period]
	if klt == 0 {
		klt = 101
	}

	// 解析指数代码
	secid := indexCode
	if strings.HasPrefix(indexCode, "sh") {
		code := strings.TrimPrefix(indexCode, "sh")
		secid = "1." + code
	} else if strings.HasPrefix(indexCode, "sz") {
		code := strings.TrimPrefix(indexCode, "sz")
		secid = "0." + code
	}

	url := fmt.Sprintf(
		"https://push2his.eastmoney.com/api/qt/stock/kline/get?secid=%s&fields1=f1,f2,f3,f4,f5,f6&fields2=f51,f52,f53,f54,f55,f56,f57,f58&klt=%d&fqt=1&end=20991231&lmt=%d",
		secid, klt, limit,
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var raw map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	data, ok := raw["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("无效的响应数据")
	}

	klinesIf, ok := data["klines"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("无K线数据")
	}

	var klines []KlineData
	for _, line := range klinesIf {
		parts, ok := line.(string)
		if !ok {
			continue
		}
		fields := strings.Split(parts, ",")
		if len(fields) < 6 {
			continue
		}

		tradeDate := fields[0]
		openTime, err := time.ParseInLocation("2006-01-02", tradeDate, time.FixedZone("CST", 8*3600))
		if err != nil {
			continue
		}
		openTime = openTime.UTC()

		klines = append(klines, KlineData{
			OpenTime:  openTime,
			CloseTime: openTime.Add(24 * time.Hour),
			Open:     parseFloatStr(fields[1]),
			Close:    parseFloatStr(fields[2]),
			High:     parseFloatStr(fields[3]),
			Low:      parseFloatStr(fields[4]),
			Volume:   parseFloatStr(fields[5]),
		})
	}

	return klines, nil
}

// parseFloatStr parses a string to float64
func parseFloatStr(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
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
