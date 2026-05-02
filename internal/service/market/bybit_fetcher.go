package market

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/smallfire/starfire/internal/config"
)

// BybitFetcher Bybit交易所行情抓取器
type BybitFetcher struct {
	client  *http.Client
	baseURL string
	config  config.MarketConfig
}

// BybitInstrumentsResp 合约信息响应
type BybitInstrumentsResp struct {
	Code    int    `json:"retCode"`
	Message string `json:"retMsg"`
	Data    struct {
		List []struct {
			Symbol     string `json:"symbol"`
			BaseCoin   string `json:"baseCoin"`
			QuoteCoin  string `json:"quoteCoin"`
			Status     string `json:"status"`
			PriceScale string `json:"priceScale"`
			LotSize    string `json:"lotSize"`
		} `json:"list"`
	} `json:"result"`
}

// BybitKlineResp K线数据响应
type BybitKlineResp struct {
	Code    int    `json:"retCode"`
	Message string `json:"retMsg"`
	Data    struct {
		List [][]interface{} `json:"list"`
	} `json:"result"`
}

// BybitTickerResp 行情响应
type BybitTickerResp struct {
	Code    int    `json:"retCode"`
	Message string `json:"retMsg"`
	Data    struct {
		List []struct {
			Symbol       string `json:"symbol"`
			LastPrice    string `json:"lastPrice"`
			Price24hPcnt string `json:"price24hPcnt"`
			HighPrice24h string `json:"highPrice24h"`
			LowPrice24h  string `json:"lowPrice24h"`
			Volume24h    string `json:"volume24h"`
			Turnover24h  string `json:"turnover24h"`
		} `json:"list"`
	} `json:"result"`
}

// NewBybitFetcher 创建Bybit抓取器
func NewBybitFetcher(cfg config.MarketConfig) *BybitFetcher {
	baseURL := "https://api.bybit.com"

	return &BybitFetcher{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: baseURL,
		config:  cfg,
	}
}

func (f *BybitFetcher) MarketCode() string {
	return "bybit"
}

func (f *BybitFetcher) SupportedPeriods() []string {
	return []string{"1", "3", "5", "15", "30", "60", "240", "D", "W", "M"}
}

func (f *BybitFetcher) FetchSymbols() ([]SymbolInfo, error) {
	// 使用 tickers 接口获取所有合约行情数据（包含 24h 成交额）
	url := fmt.Sprintf("%s/v5/market/tickers?category=linear", f.baseURL)

	resp, err := f.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result BybitTickerResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var symbols []SymbolInfo
	for _, item := range result.Data.List {
		// 过滤: 只保留 USDT 永续合约，排除到期合约
		if !strings.HasSuffix(item.Symbol, "USDT") || hasExpirationSuffix(item.Symbol) {
			continue
		}

		// 排除无成交额的合约（已暂停/未上线）
		turnover24h := parseFloat(item.Turnover24h)
		if turnover24h <= 0 {
			continue
		}

		hotScore := turnover24h

		symbols = append(symbols, SymbolInfo{
			Code:     item.Symbol,
			Name:     strings.TrimSuffix(item.Symbol, "USDT"),
			Type:     "futures",
			HotScore: hotScore,
		})
	}

	return symbols, nil
}

// hasExpirationSuffix 判断标的是否带有到期日期后缀
func hasExpirationSuffix(symbol string) bool {
	// 到期合约通常包含格式如 "-25DEC26" 的后缀
	// 其中包含 "-" 字符，并且后面跟着数字和字母组合
	for i, c := range symbol {
		if c == '-' && i > 0 && i < len(symbol)-1 {
			// 检查 "-" 后面是否包含有效的日期格式
			suffix := symbol[i+1:]
			// 简单的检查：长度应该在 6-8 字符之间，并且包含字母和数字
			if len(suffix) >= 6 && len(suffix) <= 8 {
				hasDigit := false
				hasLetter := false
				for _, s := range suffix {
					if s >= '0' && s <= '9' {
						hasDigit = true
					} else if (s >= 'A' && s <= 'Z') || (s >= 'a' && s <= 'z') {
						hasLetter = true
					}
				}
				// 如果同时包含数字和字母，很可能是到期日期后缀
				if hasDigit && hasLetter {
					return true
				}
			}
		}
	}
	return false
}

func (f *BybitFetcher) FetchKlines(symbol, period string, limit int) ([]KlineData, error) {
	url := fmt.Sprintf("%s/v5/market/kline?category=linear&symbol=%s&interval=%s&limit=%d",
		f.baseURL, symbol, period, limit)

	resp, err := f.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result BybitKlineResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return parseBybitKlines(result.Data.List, period), nil
}

// FetchKlinesByTimeRange 获取指定时间范围的K线数据
func (f *BybitFetcher) FetchKlinesByTimeRange(symbol, period string, startTime, endTime time.Time) ([]KlineData, error) {
	var allKlines []KlineData

	// Bybit API 每次最多返回1000条
	limit := 1000
	currentEnd := endTime

	for {
		start := currentEnd.Add(-time.Duration(limit) * getPeriodDuration(period))
		if start.Before(startTime) {
			start = startTime
		}

		url := fmt.Sprintf("%s/v5/market/kline?category=linear&symbol=%s&interval=%s&start=%d&end=%d&limit=%d",
			f.baseURL, symbol, period,
			start.UnixMilli(), currentEnd.UnixMilli(), limit)

		resp, err := f.client.Get(url)
		if err != nil {
			return nil, err
		}

		var result BybitKlineResp
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()

		klines := parseBybitKlines(result.Data.List, period)
		if len(klines) == 0 {
			break
		}

		allKlines = append(allKlines, klines...)

		// 如果返回的数据少于limit，说明已经取完了
		if len(klines) < limit {
			break
		}

		// 更新end时间，继续获取更早的数据
		currentEnd = klines[len(klines)-1].OpenTime.Add(-time.Second)
		if currentEnd.Before(startTime) || currentEnd.Equal(startTime) {
			break
		}

		// 防止无限循环，最多循环100次（约10万条数据）
		if len(allKlines) >= limit*100 {
			break
		}
	}

	return allKlines, nil
}

// getPeriodDuration 获取周期对应的时长
func getPeriodDuration(period string) time.Duration {
	switch period {
	case "1":
		return time.Minute
	case "3":
		return 3 * time.Minute
	case "5":
		return 5 * time.Minute
	case "15":
		return 15 * time.Minute
	case "30":
		return 30 * time.Minute
	case "60":
		return time.Hour
	case "240":
		return 4 * time.Hour
	case "D":
		return 24 * time.Hour
	case "W":
		return 7 * 24 * time.Hour
	case "M":
		return 30 * 24 * time.Hour
	default:
		return time.Minute
	}
}

func (f *BybitFetcher) FetchTicker(symbol string) (*Ticker, error) {
	url := fmt.Sprintf("%s/v5/market/tickers?category=linear&symbol=%s",
		f.baseURL, symbol)

	resp, err := f.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result BybitTickerResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Data.List) > 0 {
		tickerData := result.Data.List[0]

		lastPrice := parseFloat(tickerData.LastPrice)
		priceChange := parseFloat(tickerData.Price24hPcnt) * 100 // 转换为百分比
		high24h := parseFloat(tickerData.HighPrice24h)
		low24h := parseFloat(tickerData.LowPrice24h)
		volume24h := parseFloat(tickerData.Volume24h)

		return &Ticker{
			Symbol:      symbol,
			LastPrice:   lastPrice,
			High24h:     high24h,
			Low24h:      low24h,
			Volume24h:   volume24h,
			PriceChange: priceChange,
			ChangePct:   priceChange,
			Timestamp:   time.Now().Unix(),
		}, nil
	}

	return nil, fmt.Errorf("symbol %s not found in ticker data", symbol)
}

func parseBybitKlines(rawData [][]interface{}, period string) []KlineData {
	var klines []KlineData

	for _, item := range rawData {
		if len(item) < 6 {
			continue
		}

		timestamp, err := parseTimestamp(item[0])
		if err != nil {
			continue
		}

		open := parseFloat(item[1])
		high := parseFloat(item[2])
		low := parseFloat(item[3])
		close := parseFloat(item[4])
		volume := parseFloat(item[5])

		klines = append(klines, KlineData{
			OpenTime:  timestamp,
			CloseTime: timestamp.Add(parsePeriodDuration(period)),
			Open:      open,
			High:      high,
			Low:       low,
			Close:     close,
			Volume:    volume,
		})
	}

	return klines
}

func parseTimestamp(v interface{}) (time.Time, error) {
	switch t := v.(type) {
	case string:
		var timestamp int64
		_, err := fmt.Sscanf(t, "%d", &timestamp)
		if err != nil {
			return time.Now(), err
		}
		return time.Unix(timestamp/1000, 0).UTC(), nil
	case float64:
		return time.Unix(int64(t)/1000, 0).UTC(), nil
	case int64:
		return time.Unix(t/1000, 0).UTC(), nil
	}
	return time.Now().UTC(), nil
}

func parseFloat(v interface{}) float64 {
	switch f := v.(type) {
	case string:
		var floatVal float64
		fmt.Sscanf(f, "%f", &floatVal)
		return floatVal
	case float64:
		return f
	case int:
		return float64(f)
	case int64:
		return float64(f)
	}
	return 0
}

func parsePeriodDuration(period string) time.Duration {
	switch period {
	case "1":
		return time.Minute
	case "3":
		return 3 * time.Minute
	case "5":
		return 5 * time.Minute
	case "15":
		return 15 * time.Minute
	case "30":
		return 30 * time.Minute
	case "60":
		return time.Hour
	case "240":
		return 4 * time.Hour
	case "D":
		return 24 * time.Hour
	case "W":
		return 7 * 24 * time.Hour
	case "M":
		return 30 * 24 * time.Hour
	default:
		return time.Minute
	}
}

// FetchAStockIndices 获取A股大盘指数（Bybit不支持，返回错误）
func (f *BybitFetcher) FetchAStockIndices() ([]AStockMarketIndex, error) {
	return nil, fmt.Errorf("bybit fetcher 不支持获取A股指数")
}

// FetchSectorList 获取板块涨跌榜（Bybit不支持）
func (f *BybitFetcher) FetchSectorList(sortField string, ascending bool, limit int) ([]SectorData, error) {
	return nil, fmt.Errorf("bybit fetcher 不支持获取板块数据")
}

// FetchLimitCount 获取涨跌停统计（Bybit不支持）
func (f *BybitFetcher) FetchLimitCount() (*LimitCount, error) {
	return nil, fmt.Errorf("bybit fetcher 不支持获取涨跌停统计")
}

// FetchIndexKlines 获取指数K线数据（Bybit不支持）
func (f *BybitFetcher) FetchIndexKlines(indexCode string, period string, limit int) ([]KlineData, error) {
	return nil, fmt.Errorf("bybit fetcher 不支持获取指数K线")
}
