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
// symbol: 股票代码（如 600519）
// period: 101=日K, 102=周K, 103=月K
func (f *EastmoneyFetcher) FetchKlines(symbol, period string, limit int) ([]KlineData, error) {
	// 判断市场前缀：6开头为上海(1)，0/3开头为深圳(0)
	marketID := 0
	if len(symbol) > 0 && symbol[0] == '6' {
		marketID = 1
	}

	url := fmt.Sprintf(
		"%s/api/qt/stock/kline/get?secid=%d.%s&fields1=f1,f2,f3,f4,f5&fields2=f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61&klt=%s&fqt=1&lmt=%d",
		f.baseURL, marketID, symbol, period, limit,
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
	marketID := 0
	if len(symbol) > 0 && symbol[0] == '6' {
		marketID = 1
	}

	beg := startTime.Format("20060102")
	end := endTime.Format("20060102")

	url := fmt.Sprintf(
		"%s/api/qt/stock/kline/get?secid=%d.%s&fields1=f1,f2,f3,f4,f5&fields2=f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61&klt=%s&fqt=1&beg=%s&end=%s&lmt=1000",
		f.baseURL, marketID, symbol, period, beg, end,
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
	marketID := 0
	if len(symbol) > 0 && symbol[0] == '6' {
		marketID = 1
	}

	url := fmt.Sprintf(
		"https://push2.eastmoney.com/api/qt/stock/get?secid=%d.%s&fields=f2,f3,f4,f5,f6,f15,f16,f17,f18",
		marketID, symbol,
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

// getPeriodEndTime 根据周期计算K线收盘时间
func getPeriodEndTime(period string, openTime time.Time) time.Time {
	switch period {
	case "101": // 日K
		return time.Date(openTime.Year(), openTime.Month(), openTime.Day(), 15, 0, 0, 0, openTime.Location())
	case "102": // 周K
		closeTime := time.Date(openTime.Year(), openTime.Month(), openTime.Day(), 15, 0, 0, 0, openTime.Location())
		for closeTime.Weekday() != time.Friday {
			closeTime = closeTime.AddDate(0, 0, 1)
		}
		return closeTime
	case "103": // 月K
		return time.Date(openTime.Year(), openTime.Month()+1, 0, 15, 0, 0, 0, openTime.Location())
	default:
		return openTime.Add(24 * time.Hour)
	}
}
