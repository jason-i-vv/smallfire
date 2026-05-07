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

// InfowayFetcher Infoway A股行情抓取器
// 数据源：Infoway API，需要 API Key
type InfowayFetcher struct {
	client  *http.Client
	baseURL string
	apiKey  string
	config  config.MarketConfig
}

// InfowayBasicResp Infoway 基本面响应
type InfowayBasicResp struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data []struct {
		Symbol         string `json:"symbol"`          // 股票代码，如 000001.SZ
		NameCn         string `json:"name_cn"`         // 公司简称(中文)
		Exchange       string `json:"exchange"`        // 交易所: SZSE=深交所, SSE=上交所
		TotalShares    string `json:"total_shares"`    // 总股本
		CirculatingShares string `json:"circulating_shares"` // 流通股本
	} `json:"data"`
}

// InfowayTickResp Infoway 逐笔成交响应
type InfowayTickResp struct {
	S string `json:"s"` // 股票代码
	T int64  `json:"t"` // 时间戳(毫秒)
	P string `json:"p"` // 价格
	V string `json:"v"` // 成交量
	VW string `json:"vw"` // 成交额
	Td int    `json:"td"` // 交易日
}

// InfowayBatchKlineResp Infoway 批量K线响应
type InfowayBatchKlineResp struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data []struct {
		S        string              `json:"s"`           // 股票代码
		RespList []InfowayKlineItem `json:"respList"`    // K线数据列表
	} `json:"data"`
}

// InfowayKlineItem Infoway K线数据项
type InfowayKlineItem struct {
	T   string `json:"t"`   // 时间戳(秒)
	H   string `json:"h"`   // 最高价
	O   string `json:"o"`   // 开盘价
	L   string `json:"l"`   // 最低价
	C   string `json:"c"`   // 收盘价
	V   string `json:"v"`   // 成交量
	VW  string `json:"vw"`  // 成交额
	Pc  string `json:"pc"`  // 昨收涨跌幅
	Pca string `json:"pca"` // 昨收涨跌额
}

// InfowayOrderBookResp Infoway 五档盘口响应
type InfowayOrderBookResp struct {
	S string     `json:"s"` // 股票代码
	T int64      `json:"t"` // 时间戳(毫秒)
	A [][]string `json:"a"` // 卖单 [[价格, 数量], ...]
	B [][]string `json:"b"` // 买单 [[价格, 数量], ...]
}

// NewInfowayFetcher 创建 Infoway 抓取器
func NewInfowayFetcher(cfg config.MarketConfig) *InfowayFetcher {
	return &InfowayFetcher{
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: "https://data.infoway.io",
		apiKey:  cfg.APIKey,
		config:  cfg,
	}
}

func (f *InfowayFetcher) MarketCode() string {
	return "a_stock"
}

func (f *InfowayFetcher) SupportedPeriods() []string {
	// Infoway klineType: 1=1分钟, 2=5分钟, 3=15分钟, 4=30分钟, 5=1小时, 6=2小时, 7=4小时, 8=日K, 9=周K, 10=月K
	return []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"}
}

// klineTypeMap 周期字符串到 Infoway klineType 的映射
var klineTypeMap = map[string]int{
	"1min":    1,
	"5min":    2,
	"15min":   3,
	"30min":   4,
	"1hour":   5,
	"2hour":   6,
	"4hour":   7,
	"daily":   8,
	"weekly":  9,
	"monthly": 10,
}

// reverseKlineTypeMap Infoway klineType 到标准周期的映射
var reverseKlineTypeMap = map[int]string{
	1:  "1min",
	2:  "5min",
	3:  "15min",
	4:  "30min",
	5:  "1hour",
	6:  "2hour",
	7:  "4hour",
	8:  "daily",
	9:  "weekly",
	10: "monthly",
}

// toInfowayCode 将标准股票代码转换为 Infoway 格式
// 如 600519 -> 600519.SH, 000001 -> 000001.SZ
func toInfowayCode(symbol string) string {
	// 已经包含市场后缀的直接返回
	if strings.HasSuffix(symbol, ".SH") || strings.HasSuffix(symbol, ".SZ") {
		return symbol
	}

	// 判断市场
	// 上海: 6开头
	// 深圳: 0,3开头
	if len(symbol) > 0 && symbol[0] == '6' {
		return symbol + ".SH"
	}
	return symbol + ".SZ"
}

// fromInfowayCode 将 Infoway 格式转换回标准格式
// 如 600519.SH -> sh600519, 000001.SZ -> sz000001
func fromInfowayCode(code string) string {
	if strings.HasSuffix(code, ".SH") {
		return "sh" + strings.TrimSuffix(code, ".SH")
	}
	if strings.HasSuffix(code, ".SZ") {
		return "sz" + strings.TrimSuffix(code, ".SZ")
	}
	return code
}

// FetchSymbols 获取A股股票列表
// 通过 Infoway 基本面接口获取股票列表
func (f *InfowayFetcher) FetchSymbols() ([]SymbolInfo, error) {
	// Infoway 没有直接的股票列表接口，我们使用东方财富获取股票列表
	// 然后用 Infoway 获取每只股票的实时数据来验证和补充
	// 这里直接返回 nil，让系统使用东方财富或新浪的列表
	return nil, fmt.Errorf("infoway fetcher 不支持直接获取股票列表，请使用东方财富或新浪")
}

// FetchKlines 获取K线数据
// symbol: 股票代码（如 600519, 000001）
// period: 周期字符串 (1min, 5min, 15min, 30min, 1hour, 2hour, 4hour, daily, weekly, monthly)
// limit: 数量（最大500）
func (f *InfowayFetcher) FetchKlines(symbol, period string, limit int) ([]KlineData, error) {
	klineType, ok := klineTypeMap[period]
	if !ok {
		klineType = 8 // 默认日K
	}

	infowayCode := toInfowayCode(symbol)
	url := fmt.Sprintf("%s/stock/batch_kline/%d/%d/%s", f.baseURL, klineType, limit, infowayCode)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("apiKey", f.apiKey)

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	// Infoway 返回的是嵌套结构: {code, data: [{s, respList: [...]}]}
	var result InfowayBatchKlineResp
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("infoway kline解析失败: %w, body: %s", err, string(body))
	}

	if result.Code != 200 || len(result.Data) == 0 {
		return nil, fmt.Errorf("infoway API错误: code=%d, msg=%s", result.Code, result.Msg)
	}

	var klines []KlineData
	for _, item := range result.Data {
		for _, k := range item.RespList {
			timestamp, _ := strconv.ParseInt(k.T, 10, 64)
			openTime := time.Unix(timestamp, 0).UTC()

			open, _ := strconv.ParseFloat(k.O, 64)
			high, _ := strconv.ParseFloat(k.H, 64)
			low, _ := strconv.ParseFloat(k.L, 64)
			close, _ := strconv.ParseFloat(k.C, 64)
			volume, _ := strconv.ParseFloat(k.V, 64)
			quoteVolume, _ := strconv.ParseFloat(k.VW, 64)

			klines = append(klines, KlineData{
				OpenTime:    openTime,
				CloseTime:   openTime.Add(infowayGetPeriodDuration(period)),
				Open:        open,
				High:        high,
				Low:         low,
				Close:       close,
				Volume:      volume,
				QuoteVolume: quoteVolume,
			})
		}
	}

	return klines, nil
}

// FetchKlinesByTimeRange 获取指定时间范围的K线数据
func (f *InfowayFetcher) FetchKlinesByTimeRange(symbol, period string, startTime, endTime time.Time) ([]KlineData, error) {
	// Infoway 支持 timestamp 参数进行历史查询
	klineType, ok := klineTypeMap[period]
	if !ok {
		klineType = 8
	}

	infowayCode := toInfowayCode(symbol)
	// timestamp 为秒级时间戳
	timestamp := startTime.Unix()
	url := fmt.Sprintf("%s/stock/batch_kline/%d/%d/%s?timestamp=%d", f.baseURL, klineType, 500, infowayCode, timestamp)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("apiKey", f.apiKey)

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	var result InfowayBatchKlineResp
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("infoway kline解析失败: %w", err)
	}

	if result.Code != 200 || len(result.Data) == 0 {
		return nil, fmt.Errorf("infoway API错误: code=%d, msg=%s", result.Code, result.Msg)
	}

	var klines []KlineData
	for _, item := range result.Data {
		for _, k := range item.RespList {
			ts, _ := strconv.ParseInt(k.T, 10, 64)
			openTime := time.Unix(ts, 0).UTC()

			// 过滤时间范围之外的数据
			if openTime.Before(startTime) || openTime.After(endTime) {
				continue
			}

			klines = append(klines, KlineData{
				OpenTime:    openTime,
				CloseTime:   openTime.Add(infowayGetPeriodDuration(period)),
				Open:        parseFloatStr(k.O),
				High:        parseFloatStr(k.H),
				Low:         parseFloatStr(k.L),
				Close:       parseFloatStr(k.C),
				Volume:      parseFloatStr(k.V),
				QuoteVolume: parseFloatStr(k.VW),
			})
		}
	}

	return klines, nil
}

// FetchTicker 获取实时行情
func (f *InfowayFetcher) FetchTicker(symbol string) (*Ticker, error) {
	// Infoway 没有单独的 ticker 接口，通过 K 线获取最新数据
	klines, err := f.FetchKlines(symbol, "daily", 1)
	if err != nil {
		return nil, err
	}

	if len(klines) == 0 {
		return nil, fmt.Errorf("infoway 无行情数据: %s", symbol)
	}

	k := klines[0]

	// 解析涨跌幅
	var changePct float64
	prevClose := k.Close - (k.Close * 0) // 无法从单条K线获取涨跌额，使用昨收
	if prevClose > 0 {
		// 通过 V 和 VW 计算均价，然后获取最新价
		changePct = 0 // Infoway K线响应中没有涨跌幅字段
	}

	return &Ticker{
		Symbol:    symbol,
		LastPrice: k.Close,
		High24h:   k.High,
		Low24h:    k.Low,
		Volume24h: k.Volume,
		ChangePct: changePct,
		Timestamp: k.OpenTime.Unix(),
	}, nil
}

// FetchAStockIndices 获取A股大盘指数
func (f *InfowayFetcher) FetchAStockIndices() ([]AStockMarketIndex, error) {
	// Infoway 没有专门的指数接口，返回空让备选数据源处理
	return nil, fmt.Errorf("infoway fetcher 不支持获取A股指数")
}

// FetchSectorList 获取板块涨跌榜
func (f *InfowayFetcher) FetchSectorList(sortField string, ascending bool, limit int) ([]SectorData, error) {
	return nil, fmt.Errorf("infoway fetcher 不支持获取板块数据")
}

// FetchLimitCount 获取涨跌停统计
func (f *InfowayFetcher) FetchLimitCount() (*LimitCount, error) {
	return nil, fmt.Errorf("infoway fetcher 不支持获取涨跌停统计")
}

// FetchIndexKlines 获取指数K线数据
func (f *InfowayFetcher) FetchIndexKlines(indexCode string, period string, limit int) ([]KlineData, error) {
	// 将指数代码转换为 Infoway 格式
	// sh000001 -> 000001.SH, sz399001 -> 399001.SZ
	code := indexCode
	if strings.HasPrefix(code, "sh") {
		code = strings.TrimPrefix(code, "sh") + ".SH"
	} else if strings.HasPrefix(code, "sz") {
		code = strings.TrimPrefix(code, "sz") + ".SZ"
	}

	return f.FetchKlines(code, period, limit)
}

// infowayGetPeriodDuration 根据周期返回持续时间
func infowayGetPeriodDuration(period string) time.Duration {
	switch period {
	case "1min":
		return time.Minute
	case "5min":
		return 5 * time.Minute
	case "15min":
		return 15 * time.Minute
	case "30min":
		return 30 * time.Minute
	case "1hour":
		return time.Hour
	case "2hour":
		return 2 * time.Hour
	case "4hour":
		return 4 * time.Hour
	case "daily":
		return 24 * time.Hour
	case "weekly":
		return 7 * 24 * time.Hour
	case "monthly":
		return 30 * 24 * time.Hour
	default:
		return 24 * time.Hour
	}
}
