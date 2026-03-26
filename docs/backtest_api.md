# 回测接口设计文档

## 概述

实现了一个回测接口，允许用户对指定标的、时间范围、周期执行策略回测，并返回详细的回测结果和统计指标。

## API 接口

### 1. 执行回测

**POST** `/api/v1/backtest`

#### 请求参数

```json
{
  "symbol_code": "BTCUSDT",
  "market_code": "bybit",
  "period": "1h",
  "strategy_type": "box",
  "start_time": "2024-01-01 00:00:00",
  "end_time": "2024-12-31 23:59:59",
  "initial_capital": 100000,
  "position_size": 0.1,
  "stop_loss_pct": 0.02,
  "take_profit_pct": 0.05
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| symbol_code | string | 是 | 标的代码，如 BTCUSDT |
| market_code | string | 是 | 市场代码，支持: bybit, a_stock, us_stock |
| period | string | 是 | K线周期，支持: 15m, 1h, 1d |
| strategy_type | string | 是 | 策略类型，支持: box, trend, key_level, volume_price |
| start_time | string | 是 | 开始时间 (UTC+8)，格式: 2006-01-02 15:04:05 |
| end_time | string | 是 | 结束时间 (UTC+8)，格式: 2006-01-02 15:04:05 |
| initial_capital | float64 | 否 | 初始资金，默认: 100000 |
| position_size | float64 | 否 | 单笔仓位比例，默认: 0.1 (10%) |
| stop_loss_pct | float64 | 否 | 止损比例，默认: 0.02 (2%) |
| take_profit_pct | float64 | 否 | 止盈比例，默认: 0.05 (5%) |

#### 响应参数

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "request": { ... },
    "statistics": {
      "total_trades": 25,
      "win_trades": 15,
      "lose_trades": 10,
      "win_rate": 0.6,
      "total_pnl": 5000.00,
      "total_pnl_percent": 0.05,
      "avg_win": 500.00,
      "avg_loss": 250.00,
      "profit_factor": 2.0,
      "expectancy": 200.00,
      "max_drawdown": 1000.00,
      "max_drawdown_pct": 0.01,
      "sharpe_ratio": 1.5,
      "final_capital": 105000.00
    },
    "trades": [
      {
        "id": 1,
        "signal_id": 100,
        "entry_time": "2024-01-15T10:00:00+08:00",
        "exit_time": "2024-01-15T14:00:00+08:00",
        "direction": "long",
        "entry_price": 50000.00,
        "exit_price": 50500.00,
        "quantity": 0.2,
        "pnl": 100.00,
        "pnl_percent": 0.01,
        "fees": 8.00,
        "exit_reason": "take_profit",
        "hold_hours": 4.0,
        "cum_pnl": 100.00
      }
    ],
    "signals": [ ... ],
    "equity_curve": [
      {
        "time": "2024-01-15T10:00:00+08:00",
        "capital": 100000.00,
        "pnl": 0
      }
    ],
    "run_time_ms": 1500
  }
}
```

### 2. 获取支持的策略列表

**GET** `/api/v1/backtest/strategies`

#### 响应

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {"type": "box", "name": "box_strategy"},
    {"type": "trend", "name": "trend_strategy"},
    {"type": "key_level", "name": "key_level_strategy"},
    {"type": "volume_price", "name": "volume_price_strategy"}
  ]
}
```

### 3. 获取支持的周期列表

**GET** `/api/v1/backtest/periods`

#### 响应

```json
{
  "code": 0,
  "message": "success",
  "data": ["15m", "1h", "1d"]
}
```

## 实现文件

- `internal/models/backtest.go` - 回测请求/响应模型
- `internal/service/backtest/backtest_service.go` - 回测业务逻辑
- `internal/handler/backtest_handler.go` - HTTP 处理器

## 回测逻辑

### 1. 数据获取
- 根据标的、市场、周期、时间范围获取历史K线数据
- K线数据按时间正序排列

### 2. 策略分析
- 使用200根K线窗口进行分析
- 滑窗遍历K线数据，每根K线运行策略分析

### 3. 交易模拟
- **开仓**: 检测到 pending 状态的信号时开仓
- **平仓条件**:
  - 止损: 价格触及止损价格
  - 止盈: 价格触及止盈价格
  - 到期: 回测结束时仍未平仓
- **仓位管理**: 固定仓位比例

### 4. 统计指标
- **胜率**: 盈利交易数 / 总交易数
- **盈亏比**: 平均盈利 / 平均亏损
- **期望值**: 胜率 * 平均盈利 - 败率 * 平均亏损
- **最大回撤**: 历史最高点到当前资金的回撤
- **夏普比率**: 风险调整后收益指标

## 错误码

| 错误码 | 说明 |
|--------|------|
| 400 | 请求参数错误 |
| 500 | 服务器内部错误（数据库查询失败、K线数据不足等）|

## 示例

### cURL 示例

```bash
curl -X POST http://localhost:8080/api/v1/backtest \
  -H "Content-Type: application/json" \
  -d '{
    "symbol_code": "BTCUSDT",
    "market_code": "bybit",
    "period": "1h",
    "strategy_type": "box",
    "start_time": "2024-01-01 00:00:00",
    "end_time": "2024-03-31 23:59:59",
    "initial_capital": 100000,
    "position_size": 0.1,
    "stop_loss_pct": 0.02,
    "take_profit_pct": 0.05
  }'
```

## 更新记录

| 日期 | 版本 | 描述 |
|------|------|------|
| 2026-03-23 | v1.0 | 初始版本，实现基本回测功能 |
