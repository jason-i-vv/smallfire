package market

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/smallfire/starfire/internal/config"
)

// EastmoneyFetcher 东方财富A股行情抓取器
// 数据源：东方财富公开接口，免费无需API Key
type EastmoneyFetcher struct {
	client  *http.Client
	baseURL string
	config  config.MarketConfig
}

// EastmoneyStockListResp 股票列表响应
type EastmoneyStockListResp struct {
	Rc   int    `json:"rc"`
	Data struct {
		Total int `json:"total"`
		Diff  []struct {
			F2  interface{} `json:"f2"`  // 最新价
			F3  interface{} `json:"f3"`  // 涨跌幅
			F4  interface{} `json:"f4"`  // 涨跌额
			F5  interface{} `json:"f5"`  // 成交量
			F6  interface{} `json:"f6"`  // 成交额
			F12 string      `json:"f12"` // 股票代码
			F14 string      `json:"f14"` // 股票名称
		} `json:"diff"`
	} `json:"data"`
}

// EastmoneyKlineResp K线数据响应
type EastmoneyKlineResp struct {
	Rc   int `json:"rc"`
	Data struct {
		Code   string   `json:"code"`
		Market int      `json:"market"`
		Name   string   `json:"name"`
		Klines []string `json:"klines"`
	} `json:"data"`
}

// EastmoneyTickerResp 实时行情响应
type EastmoneyTickerResp struct {
	Rc   int `json:"rc"`
	Data struct {
		F2  interface{} `json:"f2"`  // 最新价
		F3  interface{} `json:"f3"`  // 涨跌幅
		F4  interface{} `json:"f4"`  // 涨跌额
		F5  interface{} `json:"f5"`  // 成交量
		F6  interface{} `json:"f6"`  // 成交额
		F15 interface{} `json:"f15"` // 最高价
		F16 interface{} `json:"f16"` // 最低价
		F17 interface{} `json:"f17"` // 开盘价
		F18 interface{} `json:"f18"` // 昨收
	} `json:"data"`
}

// NewEastmoneyFetcher 创建东方财富A股抓取器
func NewEastmoneyFetcher(cfg config.MarketConfig) *EastmoneyFetcher {
	return &EastmoneyFetcher{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: "https://push2his.eastmoney.com",
		config:  cfg,
	}
}

func (f *EastmoneyFetcher) MarketCode() string {
	return "a_stock"
}

func (f *EastmoneyFetcher) SupportedPeriods() []string {
	return []string{"101", "102", "103"} // 101=日K, 102=周K, 103=月K
}

// FetchSymbols 获取A股股票列表
// 通过东方财富接口获取沪深A股列表，按成交额排序取前N名
func (f *EastmoneyFetcher) FetchSymbols() ([]SymbolInfo, error) {
	// fs参数说明：
	// m:0+t:6 -> 深圳主板  m:0+t:80 -> 深圳创业板
	// m:1+t:2 -> 上海主板  m:1+t:23 -> 上海科创板
	// fid=f6 按成交额排序，po=1 降序
	url := fmt.Sprintf(
		"https://push2.eastmoney.com/api/qt/clist/get?pn=1&pz=500&po=1&np=1&ut=bd1d9ddb04089700cf9c27f6f7426281&fltt=2&invt=2&fid=f6&fs=m:0+t:6,m:0+t:80,m:1+t:2,m:1+t:23&fields=f2,f3,f6,f12,f14",
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result EastmoneyStockListResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if result.Data.Diff == nil {
		return nil, fmt.Errorf("东方财富返回数据为空")
	}

	var symbols []SymbolInfo
	for _, item := range result.Data.Diff {
		// 过滤ST股票和停牌股票
		if strings.Contains(item.F14, "ST") || strings.Contains(item.F14, "st") {
			continue
		}
		// 价格为0表示停牌
		price := parseFloat(item.F2)
		if price <= 0 {
			continue
		}

		symbols = append(symbols, SymbolInfo{
			Code:     item.F12,
			Name:     item.F14,
			Type:     "stock",
			HotScore: parseFloat(item.F6), // 用成交额作为热度分数
		})
	}

	return symbols, nil
}

// FetchKlines 获取K线数据
// symbol: 股票代码（如 600519）或指数代码（如 sh000001, sz399001）
// period: 101=日K, 102=周K, 103=月K
func (f *EastmoneyFetcher) FetchKlines(symbol, period string, limit int) ([]KlineData, error) {
	// 解析市场ID和纯代码
	marketID, pureCode := parseSymbolMarket(symbol)

	url := fmt.Sprintf(
		"%s/api/qt/stock/kline/get?secid=%d.%s&fields1=f1,f2,f3,f4,f5&fields2=f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61&klt=%s&fqt=1&lmt=%d",
		f.baseURL, marketID, pureCode, period, limit,
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result EastmoneyKlineResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return parseEastmoneyKlines(result.Data.Klines, period), nil
}

// FetchKlinesByTimeRange 获取指定时间范围的K线数据
func (f *EastmoneyFetcher) FetchKlinesByTimeRange(symbol, period string, startTime, endTime time.Time) ([]KlineData, error) {
	marketID, pureCode := parseSymbolMarket(symbol)

	beg := startTime.Format("20060102")
	end := endTime.Format("20060102")

	url := fmt.Sprintf(
		"%s/api/qt/stock/kline/get?secid=%d.%s&fields1=f1,f2,f3,f4,f5&fields2=f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61&klt=%s&fqt=1&beg=%s&end=%s&lmt=1000",
		f.baseURL, marketID, pureCode, period, beg, end,
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result EastmoneyKlineResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return parseEastmoneyKlines(result.Data.Klines, period), nil
}

// FetchTicker 获取实时行情
func (f *EastmoneyFetcher) FetchTicker(symbol string) (*Ticker, error) {
	marketID, pureCode := parseSymbolMarket(symbol)

	url := fmt.Sprintf(
		"https://push2.eastmoney.com/api/qt/stock/get?secid=%d.%s&fields=f2,f3,f4,f5,f6,f15,f16,f17,f18",
		marketID, pureCode,
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result EastmoneyTickerResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	lastPrice := parseFloat(result.Data.F2)
	changePct := parseFloat(result.Data.F3)

	return &Ticker{
		Symbol:    symbol,
		LastPrice: lastPrice,
		High24h:   parseFloat(result.Data.F15),
		Low24h:    parseFloat(result.Data.F16),
		Volume24h: parseFloat(result.Data.F5),
		ChangePct: changePct,
		Timestamp: time.Now().Unix(),
	}, nil
}

// parseEastmoneyKlines 解析东方财富K线数据
// 每行格式：日期,开盘,收盘,最高,最低,成交量,成交额,振幅,涨跌幅,涨跌额,换手率
func parseEastmoneyKlines(klines []string, period string) []KlineData {
	var result []KlineData

	for _, line := range klines {
		parts := strings.Split(line, ",")
		if len(parts) < 7 {
			continue
		}

		openTime, err := time.ParseInLocation("2006-01-02", parts[0], time.FixedZone("CST", 8*3600))
		if err != nil {
			continue
		}
		// 统一转为 UTC 存储
		openTime = openTime.UTC()

		open, _ := strconv.ParseFloat(parts[1], 64)
		close, _ := strconv.ParseFloat(parts[2], 64)
		high, _ := strconv.ParseFloat(parts[3], 64)
		low, _ := strconv.ParseFloat(parts[4], 64)
		volume, _ := strconv.ParseFloat(parts[5], 64)
		quoteVolume, _ := strconv.ParseFloat(parts[6], 64)

		result = append(result, KlineData{
			OpenTime:    openTime,
			CloseTime:   getPeriodEndTime(period, openTime),
			Open:        open,
			High:        high,
			Low:         low,
			Close:       close,
			Volume:      volume,
			QuoteVolume: quoteVolume,
		})
	}

	return result
}

// AStockMarketIndex 大盘指数结构
type AStockMarketIndex struct {
	Code       string  `json:"code"` // 1.000001, 0.399001, 0.399006
	Name       string  `json:"name"`
	Price      float64 `json:"price"`
	Change     float64 `json:"change"`      // 涨跌幅 (%)
	ChangeAmt  float64 `json:"change_amt"`  // 涨跌额
	PrevClose  float64 `json:"prev_close"`  // 昨收
}

// SectorData 板块数据
type SectorData struct {
	Code   string  `json:"code"`
	Name   string  `json:"name"`
	Change float64 `json:"change"` // 涨跌幅 (%)
}

// LimitCount 涨跌停统计
type LimitCount struct {
	UpCount   int `json:"up_count"`
	DownCount int `json:"down_count"`
}

// FetchAStockIndices 获取A股大盘指数（上证、深证、创业板）
func (f *EastmoneyFetcher) FetchAStockIndices() ([]AStockMarketIndex, error) {
	// 使用 ulist.np 接口获取多个指数数据
	url := "https://push2.eastmoney.com/api/qt/ulist.np/get?fltt=2&invt=2&fields=f2,f3,f4,f12,f14&secids=1.000001,0.399001,0.399006"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var raw map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	var result []AStockMarketIndex
	if data, ok := raw["data"].(map[string]interface{}); ok {
		if diff, ok := data["diff"].([]interface{}); ok {
			for _, item := range diff {
				if m, ok := item.(map[string]interface{}); ok {
					code := toString(m["f12"])
					name := toString(m["f14"])
					price := parseFloat(m["f2"])
					change := parseFloat(m["f3"])
					changeAmt := parseFloat(m["f4"])

					// 构建正确的 secid
					secid := code
					if len(code) == 6 && code[0] == '0' {
						secid = "0." + code
					} else if len(code) == 6 && code[0] == '1' {
						secid = "1." + code
					}

					result = append(result, AStockMarketIndex{
						Code:      secid,
						Name:      name,
						Price:     price,
						Change:    change,
						ChangeAmt: changeAmt,
					})
				}
			}
		}
	}

	return result, nil
}

// fetchIndexTicker 获取单个指数行情（已废弃，使用 FetchAStockIndices 代替）
func (f *EastmoneyFetcher) fetchIndexTicker(secid string) (*indexTicker, error) {
	return nil, fmt.Errorf("已废弃，请使用 FetchAStockIndices")
}

type indexTicker struct {
	Price     float64
	Change    float64
	ChangeAmt float64
	PrevClose float64
}

// FetchSectorList 获取板块涨跌榜
// sortField: f3=涨跌幅, f6=成交额
func (f *EastmoneyFetcher) FetchSectorList(sortField string, ascending bool, limit int) ([]SectorData, error) {
	// m:90+t:3 = 行业板块
	// fid=f3 按涨跌幅排序, po=1 降序, po=0 升序
	po := 0
	if !ascending {
		po = 1
	}
	url := fmt.Sprintf(
		"https://push2.eastmoney.com/api/qt/clist/get?pn=1&pz=%d&po=%d&np=1&ut=bd1d9ddb04089700cf9c27f6f7426281&fltt=2&invt=2&fid=%s&fs=m:90+t:3&fields=f2,f3,f12,f14",
		limit, po, sortField,
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var raw map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}

	var sectors []SectorData
	if data, ok := raw["data"].(map[string]interface{}); ok {
		if diff, ok := data["diff"].([]interface{}); ok {
			for _, item := range diff {
				if m, ok := item.(map[string]interface{}); ok {
					code := toString(m["f12"])
					name := toString(m["f14"])
					change := parseFloat(m["f3"])
					if name != "" {
						sectors = append(sectors, SectorData{
							Code:   code,
							Name:   name,
							Change: change,
						})
					}
				}
			}
		}
	}

	return sectors, nil
}

// FetchLimitCount 获取涨跌停股票数量
func (f *EastmoneyFetcher) FetchLimitCount() (*LimitCount, error) {
	// 涨停: f3 >= 9.9 (接近10%涨停限制)
	// 跌停: f3 <= -9.9
	var count LimitCount

	// 涨停数量
	urlUp := "https://push2.eastmoney.com/api/qt/clist/get?pn=1&pz=1&po=1&np=1&ut=bd1d9ddb04089700cf9c27f6f7426281&fltt=2&invt=2&fid=f3&fs=m:0+t:6,m:0+t:80,m:1+t:2,m:1+t:23&fields=f2,f3&filter=(f3>=9.9)"
	count.UpCount, _ = f.fetchLimitCountByURL(urlUp)

	// 跌停数量
	urlDown := "https://push2.eastmoney.com/api/qt/clist/get?pn=1&pz=1&po=1&np=1&ut=bd1d9ddb04089700cf9c27f6f7426281&fltt=2&invt=2&fid=f3&fs=m:0+t:6,m:0+t:80,m:1+t:2,m:1+t:23&fields=f2,f3&filter=(f3<=-9.9)"
	count.DownCount, _ = f.fetchLimitCountByURL(urlDown)

	return &count, nil
}

func (f *EastmoneyFetcher) fetchLimitCountByURL(url string) (int, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	resp, err := f.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var raw map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return 0, err
	}

	if data, ok := raw["data"].(map[string]interface{}); ok {
		if total, ok := data["total"].(float64); ok {
			return int(total), nil
		}
	}
	return 0, nil
}

// FetchIndexKlines 获取指数K线数据（用于成交量图表）
// indexCode: "sh000001" (上证), "sz399001" (深证), "sz399006" (创业板)
// period: "daily" (日K), "weekly" (周K), "monthly" (月K)
func (f *EastmoneyFetcher) FetchIndexKlines(indexCode string, period string, limit int) ([]KlineData, error) {
	// 东方财富 Kline API
	// klt=101 日K, 102 周K, 103 月K
	kltMap := map[string]int{
		"daily":   101,
		"weekly":  102,
		"monthly": 103,
	}
	klt := kltMap[period]
	if klt == 0 {
		klt = 101 // 默认日K
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

	klinesStr, ok := data["klines"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("无K线数据")
	}

	var klines []KlineData
	for _, line := range klinesStr {
		parts, ok := line.(string)
		if !ok {
			continue
		}
		// 格式: "2026-04-29,4061.82,4107.51,4112.15,4061.82,614221451,1126615371064.60,1.23"
		// 日期,开盘,收盘,最高,最低,成交量,成交额,涨跌幅
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
			Open:      parseFloatStr(fields[1]),
			Close:     parseFloatStr(fields[2]),
			High:      parseFloatStr(fields[3]),
			Low:       parseFloatStr(fields[4]),
			Volume:    parseFloatStr(fields[5]),
		})
	}

	return klines, nil
}

func toString(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func getPeriodEndTime(period string, openTime time.Time) time.Time {
	cst := time.FixedZone("CST", 8*3600)
	switch period {
	case "101": // 日K
		closeTime := time.Date(openTime.Year(), openTime.Month(), openTime.Day(), 15, 0, 0, 0, cst)
		return closeTime.UTC()
	case "102": // 周K
		closeTime := time.Date(openTime.Year(), openTime.Month(), openTime.Day(), 15, 0, 0, 0, cst)
		for closeTime.Weekday() != time.Friday {
			closeTime = closeTime.AddDate(0, 0, 1)
		}
		return closeTime.UTC()
	case "103": // 月K
		closeTime := time.Date(openTime.Year(), openTime.Month()+1, 0, 15, 0, 0, 0, cst)
		return closeTime.UTC()
	default:
		return openTime.Add(24 * time.Hour)
	}
}

// parseSymbolMarket 解析 symbol 代码，返回 (marketID, pureCode)
// 支持格式：sh000001 -> (1, "000001"), sz399001 -> (0, "399001"), 600519 -> (1, "600519")
func parseSymbolMarket(symbol string) (int, string) {
	if strings.HasPrefix(symbol, "sh") {
		return 1, strings.TrimPrefix(symbol, "sh")
	}
	if strings.HasPrefix(symbol, "sz") {
		return 0, strings.TrimPrefix(symbol, "sz")
	}
	// 纯数字代码：6开头为上海，其他为深圳
	if len(symbol) > 0 && symbol[0] == '6' {
		return 1, symbol
	}
	return 0, symbol
}
