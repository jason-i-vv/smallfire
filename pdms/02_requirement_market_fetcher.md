# 需求文档：行情抓取模块

**需求编号**: REQ-MARKET-001
**模块**: 行情抓取
**优先级**: P0
**状态**: 已完成 ✅ (基础功能)
**补充需求**: 见 REQ-MARKET-001-SUP
**完成时间**: 2024-03-22
**前置依赖**: REQ-INF-001 (基础设施)
**创建时间**: 2024-03-22

---

## 1. 需求概述

实现行情数据抓取模块，负责从各个市场获取K线数据并存储到本地数据库。

### 1.1 支持的市场

| 市场 | 数据源 | 周期 | 抓取间隔 |
|------|--------|------|----------|
| bybit | Bybit API | 15m, 1h | 60秒 |
| A股 | EastMoney API | 1d | 300秒 |
| 美股 | Yahoo Finance API | 1d | 300秒 |

### 1.2 核心功能

- 工厂模式管理多个行情抓取器
- 本地K线数据缓存
- EMA数据计算与存储
- 热度管理（标的筛选）
- 服务重启后数据恢复

---

## 2. 数据模型

### 2.1 Market 模型

```go
// internal/models/market.go
type Market struct {
    ID          int64     `json:"id"`
    MarketCode  string    `json:"market_code"`  // bybit, a_stock, us_stock
    MarketName  string    `json:"market_name"`
    MarketType  string    `json:"market_type"`  // crypto, stock
    IsEnabled   bool      `json:"is_enabled"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}
```

### 2.2 Symbol 模型

```go
// internal/models/symbol.go
type Symbol struct {
    ID            int64      `json:"id"`
    MarketID      int64      `json:"market_id"`
    MarketCode    string     `json:"market_code"`    // 关联字段
    SymbolCode    string     `json:"symbol_code"`   // BTCUSDT, 600519, NVDA
    SymbolName    string     `json:"symbol_name"`
    SymbolType    string     `json:"symbol_type"`   // spot, futures
    LastHotAt     *time.Time `json:"last_hot_at"`   // 最后一次热度更新时间
    HotScore      float64    `json:"hot_score"`     // 热度评分
    IsTracking     bool       `json:"is_tracking"`   // 是否在跟踪
    MaxKlines     int        `json:"max_klines"`    // 最大缓存K线数
    CreatedAt     time.Time  `json:"created_at"`
    UpdatedAt     time.Time  `json:"updated_at"`
}
```

### 2.3 Kline 模型

```go
// internal/models/kline.go
type Kline struct {
    ID           int64     `json:"id"`
    SymbolID     int64     `json:"symbol_id"`
    Period       string    `json:"period"`        // 1m, 5m, 15m, 1h, 4h, 1d
    OpenTime     time.Time `json:"open_time"`
    CloseTime    time.Time `json:"close_time"`
    OpenPrice    float64   `json:"open"`
    HighPrice    float64   `json:"high"`
    LowPrice     float64   `json:"low"`
    ClosePrice   float64   `json:"close"`
    Volume       float64   `json:"volume"`
    QuoteVolume  float64   `json:"quote_volume"` // 成交额
    TradesCount  int       `json:"trades_count"`
    IsClosed     bool      `json:"is_closed"`
    EMAShort     *float64  `json:"ema_short"`   // EMA30
    EMAMedium    *float64  `json:"ema_medium"`  // EMA60
    EMALong      *float64  `json:"ema_long"`    // EMA90
    CreatedAt    time.Time `json:"created_at"`
}
```

---

## 3. 行情抓取器接口

### 3.1 Fetcher 接口定义

```go
// internal/service/market/fetcher.go
type Fetcher interface {
    // 获取市场代码
    MarketCode() string

    // 获取支持的周期
    SupportedPeriods() []string

    // 获取交易对列表
    FetchSymbols() ([]SymbolInfo, error)

    // 获取K线数据
    FetchKlines(symbol string, period string, limit int) ([]KlineData, error)

    // 获取实时价格
    FetchTicker(symbol string) (*Ticker, error)
}

// 交易对信息
type SymbolInfo struct {
    Code     string  `json:"code"`
    Name     string  `json:"name"`
    Type     string  `json:"type"`    // spot, futures
    HotScore float64 `json:"hot_score"`
}

// K线数据
type KlineData struct {
    OpenTime     time.Time `json:"open_time"`
    CloseTime    time.Time `json:"close_time"`
    Open         float64   `json:"open"`
    High         float64   `json:"high"`
    Low          float64   `json:"low"`
    Close        float64   `json:"close"`
    Volume       float64   `json:"volume"`
    QuoteVolume  float64   `json:"quote_volume"`
    TradesCount  int       `json:"trades_count"`
}

// 实时行情
type Ticker struct {
    Symbol      string  `json:"symbol"`
    LastPrice   float64 `json:"last_price"`
    High24h     float64 `json:"high_24h"`
    Low24h      float64 `json:"low_24h"`
    Volume24h   float64 `json:"volume_24h"`
    QuoteVolume float64 `json:"quote_volume_24h"`
    PriceChange float64 `json:"price_change"`
    ChangePct   float64 `json:"change_pct"`
    Timestamp   int64   `json:"timestamp"`
}
```

### 3.2 工厂模式

```go
// internal/service/market/fetcher_factory.go
type Factory struct {
    fetchers map[string]Fetcher
    config   *MarketsConfig
    symbolRepo repository.SymbolRepo
    klineRepo repository.KlineRepo
}

func NewFactory(cfg *MarketsConfig, symbolRepo repository.SymbolRepo, klineRepo repository.KlineRepo) *Factory {
    f := &Factory{
        fetchers: make(map[string]Fetcher),
        config:   cfg,
        symbolRepo: symbolRepo,
        klineRepo: klineRepo,
    }

    // 注册抓取器
    if cfg.Bybit.Enabled {
        f.fetchers["bybit"] = NewBybitFetcher(cfg.Bybit)
    }
    if cfg.AStock.Enabled {
        f.fetchers["a_stock"] = NewAStockFetcher(cfg.AStock)
    }
    if cfg.USStock.Enabled {
        f.fetchers["us_stock"] = NewUSStockFetcher(cfg.USStock)
    }

    return f
}

func (f *Factory) GetFetcher(marketCode string) (Fetcher, bool) {
    fetcher, ok := f.fetchers[marketCode]
    return fetcher, ok
}

func (f *Factory) ListEnabledFetchers() []Fetcher {
    var enabled []Fetcher
    for _, fetcher := range f.fetchers {
        enabled = append(enabled, fetcher)
    }
    return enabled
}
```

---

## 4. Bybit 抓取器实现

### 4.1 API 端点

```
K线数据: GET https://api.bybit.com/v5/market/kline
交易对:  GET https://api.bybit.com/v5/market/instruments-info
实时价格: GET https://api.bybit.com/v5/market/tickers
```

### 4.2 BybitFetcher 实现

```go
// internal/service/market/bybit_fetcher.go
type BybitFetcher struct {
    client     *http.Client
    baseURL    string
    config     BybitConfig
}

func NewBybitFetcher(cfg BybitConfig) *BybitFetcher {
    baseURL := "https://api.bybit.com"
    if cfg.Testnet {
        baseURL = "https://api-testnet.bybit.com"
    }

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
    // 获取USDT合约列表
    url := fmt.Sprintf("%s/v5/market/instruments-info?category=linear", f.baseURL)

    resp, err := f.client.Get(url)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var result BybitInstrumentsResp
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }

    var symbols []SymbolInfo
    for _, item := range result.List {
        if item.QuoteCoin == "USDT" && item.Status == "Trading" {
            symbols = append(symbols, SymbolInfo{
                Code: item.Symbol,
                Name: item.BaseCoin,
                Type: "futures",
            })
        }
    }

    return symbols, nil
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

    return parseBybitKlines(result.List), nil
}
```

---

## 5. 行情同步服务

### 5.1 SyncService 实现

```go
// internal/service/market/sync_service.go
type SyncService struct {
    factory    *Factory
    klineRepo  repository.KlineRepo
    symbolRepo repository.SymbolRepo
    emaCalc    *EMACalculator
    interval   time.Duration
    stopCh     chan struct{}
    wg         sync.WaitGroup
}

func (s *SyncService) Start() {
    s.stopCh = make(chan struct{})

    // 启动每个市场的同步任务
    for _, fetcher := range s.factory.ListEnabledFetchers() {
        s.wg.Add(1)
        go s.runSyncLoop(fetcher)
    }

    // 启动热度更新任务（每小时执行一次）
    s.wg.Add(1)
    go s.runHotUpdateLoop()
}

func (s *SyncService) Stop() {
    close(s.stopCh)
    s.wg.Wait()
}

func (s *SyncService) runSyncLoop(fetcher Fetcher) {
    defer s.wg.Done()

    marketCode := fetcher.MarketCode()
    periods := fetcher.SupportedPeriods()

    // 限制periods为配置的周期
    configuredPeriods := s.getConfiguredPeriods(marketCode)
    if len(configuredPeriods) > 0 {
        periods = configuredPeriods
    }

    ticker := time.NewTicker(s.interval)
    defer ticker.Stop()

    for {
        select {
        case <-s.stopCh:
            return
        case <-ticker.C:
            // 获取需要同步的标的
            symbols, _ := s.symbolRepo.GetTrackingByMarket(marketCode)

            for _, symbol := range symbols {
                for _, period := range periods {
                    if err := s.syncSymbolKlines(symbol, fetcher, period); err != nil {
                        log.Errorf("sync %s %s %s failed: %v", marketCode, symbol.Code, period, err)
                    }
                }
            }
        }
    }
}

func (s *SyncService) syncSymbolKlines(symbol *model.Symbol, fetcher Fetcher, period string) error {
    // 获取最新K线
    klines, err := fetcher.FetchKlines(symbol.Code, s.mapPeriod(period), 100)
    if err != nil {
        return err
    }

    // 存储到数据库
    for _, k := range klines {
        // 检查是否已存在
        exists, _ := s.klineRepo.Exists(symbol.ID, period, k.OpenTime)
        if exists {
            continue
        }

        // 计算EMA
        kline := s.convertToModel(symbol.ID, period, k)
        s.calculateEMA(kline)

        if err := s.klineRepo.Create(kline); err != nil {
            log.Errorf("create kline failed: %v", err)
        }
    }

    return nil
}
```

---

## 6. EMA 计算服务

### 6.1 EMACalculator 实现

```go
// internal/service/ema/ema_calculator.go
type EMACalculator struct {
    periods []int // [30, 60, 90]
}

func NewEMACalculator(periods []int) *EMACalculator {
    if len(periods) == 0 {
        periods = []int{30, 60, 90}
    }
    return &EMACalculator{periods: periods}
}

// 计算EMA
// EMA = (Close - EMA_prev) * multiplier + EMA_prev
// multiplier = 2 / (period + 1)
func (e *EMACalculator) Calculate(klines []model.Kline) []model.Kline {
    if len(klines) == 0 {
        return klines
    }

    // 按时间排序
    sort.Slice(klines, func(i, j int) bool {
        return klines[i].OpenTime.Before(klines[j].OpenTime)
    })

    for _, period := range e.periods {
        ema := e.calculateSingleEMA(klines, period)
        for i, v := range ema {
            switch period {
            case 30:
                klines[i].EMAShort = &v
            case 60:
                klines[i].EMAMedium = &v
            case 90:
                klines[i].EMALong = &v
            }
        }
    }

    return klines
}

func (e *EMACalculator) calculateSingleEMA(klines []model.Kline, period int) []float64 {
    if len(klines) < period {
        return make([]float64, len(klines))
    }

    result := make([]float64, len(klines))
    multiplier := 2.0 / float64(period+1)

    // 初始SMA作为第一个EMA
    var sma float64
    for i := 0; i < period; i++ {
        sma += klines[i].ClosePrice
    }
    sma /= float64(period)

    for i := 0; i < len(klines); i++ {
        if i < period-1 {
            continue // 数据不足，用0表示
        } else if i == period-1 {
            result[i] = sma
        } else {
            result[i] = (klines[i].ClosePrice-result[i-1])*multiplier + result[i-1]
        }
    }

    return result
}
```

---

## 7. 热度管理

### 7.1 HotManager 实现

```go
// internal/service/market/hot_manager.go
type HotManager struct {
    symbolRepo  repository.SymbolRepo
    marketRepo  repository.MarketRepo
    factory     *Factory
    config      *MarketsConfig
}

func (m *HotManager) UpdateHotSymbols() error {
    for _, fetcher := range m.factory.ListEnabledFetchers() {
        if err := m.updateMarketHot(fetcher); err != nil {
            log.Errorf("update hot symbols failed for %s: %v", fetcher.MarketCode(), err)
        }
    }
    return nil
}

func (m *HotManager) updateMarketHot(fetcher Fetcher) error {
    marketCode := fetcher.MarketCode()
    limit := m.getLimit(marketCode)
    hotDays := m.getHotDays(marketCode)

    // 获取所有交易对
    symbols, err := fetcher.FetchSymbols()
    if err != nil {
        return err
    }

    // 按热度排序
    sort.Slice(symbols, func(i, j int) bool {
        return symbols[i].HotScore > symbols[j].HotScore
    })

    // 取前N名
    if len(symbols) > limit {
        symbols = symbols[:limit]
    }

    // 更新数据库
    now := time.Now()
    for _, sym := range symbols {
        // 查找或创建标的
        symbol, err := m.symbolRepo.FindByCode(marketCode, sym.Code)
        if err != nil {
            // 创建新标的
            symbol = &model.Symbol{
                MarketID:   m.getMarketID(marketCode),
                SymbolCode: sym.Code,
                SymbolName: sym.Name,
                SymbolType: sym.Type,
                IsTracking: true,
                MaxKlines: 1000,
            }
            m.symbolRepo.Create(symbol)
        }

        // 更新热度
        symbol.HotScore = sym.HotScore
        symbol.LastHotAt = &now
        symbol.IsTracking = true
        m.symbolRepo.Update(symbol)
    }

    // 清理过期标的（超过N天无热度更新）
    cutoff := now.AddDate(0, 0, -hotDays)
    m.symbolRepo.DisableExpiredHot(cutoff)

    return nil
}
```

---

## 8. K线查询接口

### 8.1 查询服务

```go
// internal/service/market/kline_service.go
type KlineService struct {
    klineRepo  repository.KlineRepo
    fetcher    *Factory
}

func (s *KlineService) GetKlines(symbolID int64, period string, startTime, endTime *time.Time, limit int) ([]model.Kline, error) {
    // 1. 先查本地数据库
    klines, err := s.klineRepo.GetBySymbolPeriod(symbolID, period, startTime, endTime, limit)
    if err != nil {
        return nil, err
    }

    // 2. 如果本地数据不足，补充从API获取
    if len(klines) < limit {
        // 获取缺失的数据范围
        // 调用API获取
        // 合并数据
    }

    return klines, nil
}

func (s *KlineService) GetLatestKline(symbolID int64, period string) (*model.Kline, error) {
    return s.klineRepo.GetLatest(symbolID, period)
}
```

---

## 9. 周期映射

### 9.1 周期转换

```go
// internal/service/market/period_mapper.go
var PeriodMap = map[string]map[string]string{
    "bybit": {
        "1m":  "1",
        "5m":  "5",
        "15m": "15",
        "30m": "30",
        "1h":  "60",
        "4h":  "240",
        "1d":  "D",
    },
    "a_stock": {
        "1d": "101",
    },
    "us_stock": {
        "1d": "1d",
    },
}

func MapPeriod(marketCode, period string) string {
    if m, ok := PeriodMap[marketCode]; ok {
        if p, ok := m[period]; ok {
            return p
        }
    }
    return period
}
```

---

## 10. 文件结构

```
internal/service/market/
├── fetcher.go           # Fetcher接口定义
├── fetcher_factory.go   # 工厂模式
├── bybit_fetcher.go     # Bybit抓取器
├── a_stock_fetcher.go   # A股抓取器
├── us_stock_fetcher.go  # 美股抓取器
├── sync_service.go      # 同步服务
├── kline_service.go     # K线查询服务
├── hot_manager.go        # 热度管理
└── period_mapper.go     # 周期映射

internal/service/ema/
└── ema_calculator.go    # EMA计算
```

---

## 11. 配置项

### 11.1 markets 配置

```yaml
markets:
  bybit:
    enabled: true
    api_key: ""           # 可选，用于私有接口
    api_secret: ""        # 可选
    testnet: false
    symbols_limit: 200    # 最多跟踪的交易对数量
    hot_days: 30          # 热度有效期（天）
    periods: ["15", "60"] # API周期值
    fetch_interval: 60    # 抓取间隔（秒）

  a_stock:
    enabled: true
    symbols_limit: 200
    hot_days: 30
    periods: ["101"]
    fetch_interval: 300

  us_stock:
    enabled: true
    symbols_limit: 200
    hot_days: 30
    periods: ["1d"]
    fetch_interval: 300

ema:
  periods: [30, 60, 90]  # EMA计算周期
```

---

## 12. 验收标准

### 12.1 功能验收

- [ ] BybitFetcher 能正确获取K线数据
- [ ] 工厂模式能正确管理多个抓取器
- [ ] K线数据能正确存储到数据库
- [ ] EMA计算结果正确
- [ ] 热度管理能按配置数量筛选标的
- [ ] 同步服务能定期更新数据

### 12.2 数据验收

- [ ] K线数据完整性（无缺失）
- [ ] EMA数值计算正确（与tradingview对比验证）
- [ ] 时间戳使用 UTC+8 时区
- [ ] 数据唯一性约束生效

### 12.3 性能验收

- [ ] 单次API调用超时控制在30秒内
- [ ] 并发抓取不会触发API限流
- [ ] 数据库批量写入效率正常

---

## 13. 注意事项

1. **API限流**：Bybit API有频率限制，需要实现请求间隔控制
2. **数据一致性**：K线数据使用 (symbol_id, period, open_time) 作为唯一键
3. **时间时区**：所有时间统一使用 UTC+8 时区存储和返回
4. **错误重试**：网络异常时实现指数退避重试
5. **优雅关闭**：服务停止时等待正在处理的请求完成

---

**前置依赖**: REQ-INF-001
**执行人**: 待分配
**预计工时**: 6小时
**实际完成时间**: 待填写
