package market

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/smallfire/starfire/internal/config"
)

// YahooFetcher Yahoo Finance 美股行情抓取器
// 数据源：Yahoo Finance v8 chart 公开接口，免费无需API Key
type YahooFetcher struct {
	client  *http.Client
	baseURL string
	config  config.MarketConfig
}

// YahooChartMeta Yahoo Finance chart meta 信息
type YahooChartMeta struct {
	Symbol               string  `json:"symbol"`
	Currency             string  `json:"currency"`
	ExchangeName         string  `json:"exchangeName"`
	FullExchangeName     string  `json:"fullExchangeName"`
	InstrumentType       string  `json:"instrumentType"`
	RegularMarketPrice   float64 `json:"regularMarketPrice"`
	ShortName            string  `json:"shortName"`
	LongName             string  `json:"longName"`
	FiftyTwoWeekHigh     float64 `json:"fiftyTwoWeekHigh"`
	FiftyTwoWeekLow      float64 `json:"fiftyTwoWeekLow"`
	RegularMarketDayHigh float64 `json:"regularMarketDayHigh"`
	RegularMarketDayLow  float64 `json:"regularMarketDayLow"`
	RegularMarketVolume  float64 `json:"regularMarketVolume"`
}

// YahooQuoteData Yahoo Finance OHLCV 数据
type YahooQuoteData struct {
	Open   []interface{} `json:"open"`
	High   []interface{} `json:"high"`
	Low    []interface{} `json:"low"`
	Close  []interface{} `json:"close"`
	Volume []interface{} `json:"volume"`
}

// YahooChartResult Yahoo Finance chart result
type YahooChartResult struct {
	Meta       YahooChartMeta   `json:"meta"`
	Timestamp  []int64          `json:"timestamp"`
	Indicators struct {
		Quote []YahooQuoteData `json:"quote"`
	} `json:"indicators"`
}

// YahooChartResp Yahoo Finance chart 响应
type YahooChartResp struct {
	Chart struct {
		Result []YahooChartResult `json:"result"`
		Error  interface{}        `json:"error"`
	} `json:"chart"`
}

// YahooScreenerResp Yahoo Finance 股票筛选器响应（用于获取热门美股列表）
type YahooScreenerResp struct {
	Finance struct {
		Result []struct {
			Quotes []struct {
				Symbol      string  `json:"symbol"`
				ShortName   string  `json:"shortName"`
				LongName    string  `json:"longName"`
				RegularMarketPrice   float64 `json:"regularMarketPrice"`
				RegularMarketVolume  float64 `json:"regularMarketVolume"`
				AverageDailyVolume3Month float64 `json:"averageDailyVolume3Month"`
			} `json:"quotes"`
		} `json:"result"`
	} `json:"finance"`
}

// NewYahooFetcher 创建Yahoo Finance美股抓取器
func NewYahooFetcher(cfg config.MarketConfig) *YahooFetcher {
	return &YahooFetcher{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: "https://query1.finance.yahoo.com",
		config:  cfg,
	}
}

func (f *YahooFetcher) MarketCode() string {
	return "us_stock"
}

func (f *YahooFetcher) SupportedPeriods() []string {
	return []string{"1d", "1wk", "1mo"} // 日K, 周K, 月K
}

// FetchSymbols 获取美股热门股票列表
// 通过Yahoo Finance筛选接口获取成交量最大的热门美股
func (f *YahooFetcher) FetchSymbols() ([]SymbolInfo, error) {
	// 获取成交量最大的美股
	url := "https://query1.finance.yahoo.com/v1/finance/screener/predefined/saved?scrIds=most_actives&count=500&formType=RAW"

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

	var result YahooScreenerResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Finance.Result) == 0 || len(result.Finance.Result[0].Quotes) == 0 {
		return nil, fmt.Errorf("Yahoo Finance 返回热门股票数据为空")
	}

	var symbols []SymbolInfo
	for _, quote := range result.Finance.Result[0].Quotes {
		// 过滤ETF和指数
		if quote.RegularMarketPrice <= 0 {
			continue
		}

		name := quote.ShortName
		if name == "" {
			name = quote.LongName
		}

		symbols = append(symbols, SymbolInfo{
			Code:     quote.Symbol,
			Name:     name,
			Type:     "stock",
			HotScore: float64(quote.RegularMarketVolume),
		})
	}

	return symbols, nil
}

// FetchKlines 获取K线数据
// symbol: 股票代码（如 AAPL）
// period: 1d=日K, 1wk=周K, 1mo=月K
func (f *YahooFetcher) FetchKlines(symbol, period string, limit int) ([]KlineData, error) {
	// Yahoo API 的 interval 参数需要映射
	interval := period
	switch period {
	case "1d":
		interval = "1d"
	case "1wk":
		interval = "1wk"
	case "1mo":
		interval = "1mo"
	}

	// 计算开始时间（获取足够多的K线）
	daysBack := limit * 2
	switch period {
	case "1d":
		daysBack = limit * 2
	case "1wk":
		daysBack = limit * 10
	case "1mo":
		daysBack = limit * 40
	}
	// 至少回溯2年
	if daysBack < 730 {
		daysBack = 730
	}
	startTime := time.Now().AddDate(0, 0, -daysBack).Unix()
	endTime := time.Now().Unix()

	url := fmt.Sprintf(
		"%s/v8/finance/chart/%s?period1=%d&period2=%d&interval=%s&events=history",
		f.baseURL, symbol, startTime, endTime, interval,
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

	var result YahooChartResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Chart.Result) == 0 {
		return nil, fmt.Errorf("Yahoo Finance 未返回 %s 的K线数据", symbol)
	}

	klines := parseYahooKlines(result.Chart.Result[0], period)

	// 只返回最近 limit 条
	if len(klines) > limit {
		klines = klines[len(klines)-limit:]
	}

	return klines, nil
}

// FetchKlinesByTimeRange 获取指定时间范围的K线数据
func (f *YahooFetcher) FetchKlinesByTimeRange(symbol, period string, startTime, endTime time.Time) ([]KlineData, error) {
	interval := period
	switch period {
	case "1d":
		interval = "1d"
	case "1wk":
		interval = "1wk"
	case "1mo":
		interval = "1mo"
	}

	url := fmt.Sprintf(
		"%s/v8/finance/chart/%s?period1=%d&period2=%d&interval=%s&events=history",
		f.baseURL, symbol, startTime.Unix(), endTime.Unix(), interval,
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

	var result YahooChartResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Chart.Result) == 0 {
		return nil, fmt.Errorf("Yahoo Finance 未返回 %s 的K线数据", symbol)
	}

	return parseYahooKlines(result.Chart.Result[0], period), nil
}

// FetchTicker 获取实时行情
func (f *YahooFetcher) FetchTicker(symbol string) (*Ticker, error) {
	startTime := time.Now().AddDate(0, 0, -5).Unix()
	endTime := time.Now().Unix()

	url := fmt.Sprintf(
		"%s/v8/finance/chart/%s?period1=%d&period2=%d&interval=1d&events=history",
		f.baseURL, symbol, startTime, endTime,
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

	var result YahooChartResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Chart.Result) == 0 {
		return nil, fmt.Errorf("Yahoo Finance 未返回 %s 的行情数据", symbol)
	}

	meta := result.Chart.Result[0].Meta
	lastPrice := meta.RegularMarketPrice

	// 计算涨跌幅
	changePct := 0.0
	quotes := result.Chart.Result[0].Indicators.Quote
	if len(quotes) > 0 && len(quotes[0].Close) >= 2 {
		prevClose := parseFloat(quotes[0].Close[len(quotes[0].Close)-2])
		if prevClose > 0 {
			changePct = (lastPrice - prevClose) / prevClose * 100
		}
	}

	return &Ticker{
		Symbol:    symbol,
		LastPrice: lastPrice,
		High24h:   meta.RegularMarketDayHigh,
		Low24h:    meta.RegularMarketDayLow,
		Volume24h: meta.RegularMarketVolume,
		ChangePct: changePct,
		Timestamp: time.Now().Unix(),
	}, nil
}

// parseYahooKlines 解析Yahoo Finance K线数据
func parseYahooKlines(result YahooChartResult, period string) []KlineData {
	quotes := result.Indicators.Quote
	if len(quotes) == 0 {
		return nil
	}
	quote := quotes[0]

	if len(result.Timestamp) == 0 {
		return nil
	}

	var klines []KlineData
	for i, ts := range result.Timestamp {
		openTime := time.Unix(ts, 0)

		openVal := parseFloatIndex(quote.Open, i)
		highVal := parseFloatIndex(quote.High, i)
		lowVal := parseFloatIndex(quote.Low, i)
		closeVal := parseFloatIndex(quote.Close, i)
		volumeVal := parseFloatIndex(quote.Volume, i)

		// 跳过数据不完整的K线
		if openVal == 0 && highVal == 0 && lowVal == 0 && closeVal == 0 {
			continue
		}

		closeTime := getYahooPeriodEndTime(period, openTime)

		klines = append(klines, KlineData{
			OpenTime:  openTime,
			CloseTime: closeTime,
			Open:      openVal,
			High:      highVal,
			Low:       lowVal,
			Close:     closeVal,
			Volume:    volumeVal,
		})
	}

	return klines
}

// parseFloatIndex 安全地从interface{}数组中取float64
func parseFloatIndex(arr []interface{}, idx int) float64 {
	if idx >= len(arr) || arr[idx] == nil {
		return 0
	}
	return parseFloat(arr[idx])
}

// getYahooPeriodEndTime 根据周期计算K线收盘时间
func getYahooPeriodEndTime(period string, openTime time.Time) time.Time {
	// Yahoo返回的时间戳是UTC，美东时间16:00收盘
	loc := time.FixedZone("ET", -5*3600)
	switch period {
	case "1d":
		// 日K：当天美东时间16:00
		return time.Date(openTime.Year(), openTime.Month(), openTime.Day(), 16, 0, 0, 0, loc)
	case "1wk":
		// 周K：当天美东时间16:00
		return time.Date(openTime.Year(), openTime.Month(), openTime.Day(), 16, 0, 0, 0, loc)
	case "1mo":
		// 月K：当天美东时间16:00
		return time.Date(openTime.Year(), openTime.Month(), openTime.Day(), 16, 0, 0, 0, loc)
	default:
		return openTime.Add(24 * time.Hour)
	}
}
