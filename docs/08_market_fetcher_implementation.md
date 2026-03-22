# 行情抓取模块实现文档

## 1. 概述

本文档详细记录了行情抓取模块的实现细节。该模块负责从各个交易市场（Bybit、A股、美股）抓取K线数据，并提供本地缓存、EMA计算、热度管理等功能。

## 2. 架构设计

### 2.1 核心组件

- **Fetcher接口**：定义行情抓取器的统一接口
- **BybitFetcher**：Bybit交易所的具体实现
- **Factory**：工厂模式管理多个行情抓取器
- **SyncService**：定时同步服务，负责定期更新K线数据
- **HotManager**：热度管理，负责标的筛选
- **KlineService**：K线查询服务
- **EMACalculator**：EMA指标计算

### 2.2 文件结构

```
internal/service/market/
├── fetcher.go           # Fetcher接口定义
├── fetcher_factory.go   # 工厂模式
├── bybit_fetcher.go     # Bybit抓取器
├── sync_service.go      # 同步服务
├── kline_service.go     # K线查询服务
├── hot_manager.go        # 热度管理
└── period_mapper.go     # 周期映射

internal/service/ema/
└── ema_calculator.go    # EMA计算
```

## 3. 接口设计

### 3.1 Fetcher接口

```go
// Fetcher 行情抓取器接口
type Fetcher interface {
	MarketCode() string
	SupportedPeriods() []string
	FetchSymbols() ([]SymbolInfo, error)
	FetchKlines(symbol string, period string, limit int) ([]KlineData, error)
	FetchTicker(symbol string) (*Ticker, error)
}
```

### 3.2 Factory工厂模式

```go
type Factory struct {
	fetchers map[string]Fetcher
	config   *config.MarketsConfig
	symbolRepo repository.SymbolRepo
	klineRepo  repository.KlineRepo
}

func NewFactory(cfg *config.MarketsConfig, symbolRepo repository.SymbolRepo, klineRepo repository.KlineRepo) *Factory
func (f *Factory) GetFetcher(marketCode string) (Fetcher, bool)
func (f *Factory) ListEnabledFetchers() []Fetcher
```

## 4. 主要功能实现

### 4.1 Bybit抓取器

Bybit抓取器实现了以下功能：
- 获取合约列表
- 抓取K线数据（支持多种周期）
- 获取实时行情
- 处理API响应

### 4.2 同步服务

SyncService负责定期同步K线数据：
- 每60秒同步一次
- 按市场、标的、周期维度进行同步
- 增量更新，避免重复数据
- 优雅关闭机制

### 4.3 热度管理

HotManager负责管理热度标的：
- 每小时更新一次
- 支持配置筛选数量和有效期
- 自动清理过期标的

### 4.4 EMA计算

EMACalculator实现了EMA指标计算：
- 支持30、60、90周期
- 增量计算，提升性能
- 与tradingview计算结果一致

## 5. 配置说明

### 5.1 市场配置

```yaml
markets:
  bybit:
    enabled: true
    api_key: ""
    api_secret: ""
    testnet: false
    symbols_limit: 200
    hot_days: 30
    periods: ["15m", "1h"]
    fetch_interval: 60
  a_stock:
    enabled: false
    symbols_limit: 200
    hot_days: 30
    periods: ["1d"]
    fetch_interval: 300
  us_stock:
    enabled: false
    symbols_limit: 200
    hot_days: 30
    periods: ["1d"]
    fetch_interval: 300
```

### 5.2 EMA配置

```yaml
ema:
  periods: [30, 60, 90]
```

## 6. 数据库设计

### 6.1 表结构

- **markets**：市场信息表
- **symbols**：标的信息表
- **klines**：K线数据表

### 6.2 初始化脚本

初始化脚本位于`db-scripts/001_init_tables.sql`

## 7. 启动与运行

### 7.1 服务启动

```bash
# 开发模式
make dev

# Docker 构建
make docker-build-amd64

# Docker Compose 部署
docker-compose up -d
```

### 7.2 配置环境变量

```bash
cp .env.example .env
# 编辑 .env 文件，配置数据库和其他参数
```

## 8. API接口

### 8.1 健康检查

```bash
GET /health
GET /api/v1/health
```

## 9. 测试验证

### 9.1 本地开发

```bash
go run cmd/server/main.go config/config.yml
```

### 9.2 单元测试

```bash
go test ./internal/service/market -v
go test ./internal/service/ema -v
```

## 10. 注意事项

### 10.1 API限流

- Bybit API有请求频率限制，需要控制并发
- 实现了请求超时和重试机制

### 10.2 数据一致性

- 使用(symbol_id, period, open_time)作为唯一键
- 时间戳统一使用UTC+8时区

### 10.3 错误处理

- 网络异常时实现指数退避重试
- 服务停止时等待正在处理的请求完成

## 11. 版本信息

- **版本**：v1.0.0
- **日期**：2024-03-22
- **状态**：已完成
