# 星火量化系统 - API接口设计

## 1. API 概览

### 1.1 基础信息

```
Base URL: http://localhost:8080/api
Content-Type: application/json
认证方式: JWT Bearer Token
```

### 1.2 通用响应格式

```json
{
  "code": 0,           // 状态码，0表示成功
  "message": "success", // 消息
  "data": {},          // 数据
  "timestamp": 1234567890
}
```

### 1.3 错误码

| 错误码 | 说明 |
|--------|------|
| 0 | 成功 |
| 1001 | 参数错误 |
| 1002 | 认证失败 |
| 1003 | 无权限 |
| 2001 | 资源不存在 |
| 3001 | 服务器内部错误 |

## 2. 认证接口

### 2.1 用户登录

```
POST /api/auth/login
```

**请求参数：**
```json
{
  "username": "admin",
  "password": "password123"
}
```

**响应：**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIs...",
    "expires_at": "2024-03-16T00:00:00Z",
    "user": {
      "id": 1,
      "username": "admin",
      "nickname": "管理员"
    }
  }
}
```

### 2.2 获取当前用户

```
GET /api/auth/me
```

**请求头：**
```
Authorization: Bearer <token>
```

## 3. 市场接口

### 3.1 获取市场列表

```
GET /api/markets
```

**响应：**
```json
{
  "code": 0,
  "data": [
    {
      "id": 1,
      "market_code": "bybit",
      "market_name": "Bybit交易所",
      "market_type": "crypto",
      "is_enabled": true
    }
  ]
}
```

### 3.2 获取标的列表

```
GET /api/markets/{market_code}/symbols
```

**查询参数：**
| 参数 | 类型 | 说明 |
|------|------|------|
| page | int | 页码 |
| page_size | int | 每页数量 |
| is_tracking | bool | 是否在跟踪 |

### 3.3 获取K线数据

```
GET /api/klines
```

**查询参数：**
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| symbol_id | int | 是 | 标的ID |
| period | string | 是 | 周期：1m,5m,15m,1h,4h,1d |
| start_time | string | 否 | 开始时间 (ISO8601) |
| end_time | string | 否 | 结束时间 (ISO8601) |
| limit | int | 否 | 数量限制，默认500 |

**响应：**
```json
{
  "code": 0,
  "data": {
    "symbol_id": 1,
    "symbol_code": "BTCUSDT",
    "period": "15m",
    "klines": [
      {
        "id": 12345,
        "open_time": "2024-03-15T14:00:00Z",
        "close_time": "2024-03-15T14:15:00Z",
        "open": 67200.50,
        "high": 67350.00,
        "low": 67150.00,
        "close": 67245.50,
        "volume": 125.45,
        "quote_volume": 8432150.25,
        "ema_short": 67210.30,
        "ema_medium": 67185.60,
        "ema_long": 67150.00
      }
    ]
  }
}
```

## 4. 信号接口

### 4.1 获取信号列表

```
GET /api/signals
```

**查询参数：**
| 参数 | 类型 | 说明 |
|------|------|------|
| market_code | string | 市场代码 |
| signal_type | string | 信号类型 |
| direction | string | 方向：long, short |
| strength | int | 强度：1,2,3 |
| status | string | 状态 |
| start_date | string | 开始日期 |
| end_date | string | 结束日期 |
| page | int | 页码 |
| page_size | int | 每页数量 |

**响应：**
```json
{
  "code": 0,
  "data": {
    "total": 128,
    "page": 1,
    "page_size": 20,
    "items": [
      {
        "id": 1001,
        "symbol": {
          "id": 1,
          "code": "BTCUSDT",
          "name": "Bitcoin"
        },
        "market": {
          "code": "bybit",
          "name": "Bybit"
        },
        "signal_type": "box_breakout",
        "source_type": "box",
        "direction": "long",
        "strength": 3,
        "price": 67345.00,
        "target_price": 68500.00,
        "stop_loss_price": 66500.00,
        "period": "15m",
        "status": "pending",
        "created_at": "2024-03-15T14:35:00Z"
      }
    ]
  }
}
```

### 4.2 获取信号详情

```
GET /api/signals/{id}
```

### 4.3 创建交易跟踪

```
POST /api/signals/{id}/track
```

**响应：**
```json
{
  "code": 0,
  "data": {
    "track_id": 501,
    "signal_id": 1001,
    "entry_price": null,
    "status": "pending"
  }
}
```

## 5. 交易跟踪接口

### 5.1 获取当前持仓

```
GET /api/positions
```

**响应：**
```json
{
  "code": 0,
  "data": [
    {
      "id": 501,
      "signal_id": 1001,
      "symbol": {
        "id": 1,
        "code": "BTCUSDT",
        "name": "Bitcoin"
      },
      "direction": "long",
      "entry_price": 67345.00,
      "entry_time": "2024-03-15T14:36:00Z",
      "quantity": 0.15,
      "position_value": 10101.75,
      "current_price": 67500.00,
      "unrealized_pnl": 232.50,
      "unrealized_pnl_pct": 2.30,
      "stop_loss_price": 66500.00,
      "take_profit_price": 68500.00,
      "created_at": "2024-03-15T14:36:00Z"
    }
  ]
}
```

### 5.2 平仓

```
POST /api/positions/{id}/close
```

**请求参数：**
```json
{
  "reason": "manual",
  "exit_price": 67450.00
}
```

### 5.3 获取历史交易

```
GET /api/trades
```

**查询参数：**
| 参数 | 类型 | 说明 |
|------|------|------|
| status | string | 状态：open, closed |
| direction | string | 方向 |
| start_date | string | 开始日期 |
| end_date | string | 结束日期 |
| page | int | 页码 |
| page_size | int | 每页数量 |

## 6. 统计接口

### 6.1 获取统计数据

```
GET /api/trades/stats
```

**查询参数：**
| 参数 | 类型 | 说明 |
|------|------|------|
| start_date | string | 开始日期 |
| end_date | string | 结束日期 |
| market_code | string | 市场代码 |
| signal_type | string | 信号类型 |

**响应：**
```json
{
  "code": 0,
  "data": {
    "summary": {
      "total_trades": 128,
      "win_trades": 75,
      "loss_trades": 53,
      "win_rate": 58.59,
      "total_pnl": 12450.00,
      "avg_win": 386.50,
      "avg_loss": 208.30,
      "profit_factor": 1.85
    },
    "risk": {
      "max_drawdown": 320.00,
      "max_drawdown_pct": 3.2,
      "sharpe_ratio": 2.15,
      "calmar_ratio": 3.86
    },
    "timing": {
      "avg_holding_hours": 4.2,
      "trades_per_day": 4.3
    }
  }
}
```

### 6.2 获取权益曲线

```
GET /api/equity
```

**查询参数：**
| 参数 | 类型 | 说明 |
|------|------|------|
| start_date | string | 开始日期 |
| end_date | string | 结束日期 |
| interval | string | 间隔：hour, day, week |

**响应：**
```json
{
  "code": 0,
  "data": {
    "equity_curve": [
      {
        "timestamp": "2024-03-01T00:00:00Z",
        "equity": 100000.00
      },
      {
        "timestamp": "2024-03-02T00:00:00Z",
        "equity": 100450.00
      }
    ],
    "drawdown_curve": [
      {
        "timestamp": "2024-03-05T00:00:00Z",
        "drawdown": 0.0
      },
      {
        "timestamp": "2024-03-06T00:00:00Z",
        "drawdown": -320.00
      }
    ]
  }
}
```

### 6.3 获取信号分析

```
GET /api/signals/analysis
```

**响应：**
```json
{
  "code": 0,
  "data": {
    "by_type": [
      {
        "signal_type": "box_breakout",
        "count": 45,
        "win_count": 28,
        "win_rate": 62.22,
        "total_pnl": 8500.00
      }
    ],
    "by_market": [
      {
        "market_code": "bybit",
        "count": 100,
        "win_rate": 60.00
      }
    ],
    "by_direction": [
      {
        "direction": "long",
        "count": 85,
        "win_rate": 61.18
      },
      {
        "direction": "short",
        "count": 43,
        "win_rate": 53.49
      }
    ]
  }
}
```

## 7. 箱体数据接口

### 7.1 获取活跃箱体

```
GET /api/boxes
```

**查询参数：**
| 参数 | 类型 | 说明 |
|------|------|------|
| symbol_id | int | 标的ID |
| status | string | 状态：active, closed |
| period | string | 周期 |

**响应：**
```json
{
  "code": 0,
  "data": [
    {
      "id": 201,
      "symbol": {
        "id": 1,
        "code": "BTCUSDT"
      },
      "box_type": "consolidation",
      "status": "active",
      "high_price": 67350.00,
      "low_price": 66105.00,
      "width_price": 1245.00,
      "width_percent": 1.89,
      "klines_count": 32,
      "start_time": "2024-03-15T06:00:00Z",
      "subscriber_count": 1,
      "created_at": "2024-03-15T14:00:00Z"
    }
  ]
}
```

### 7.2 获取箱体详情

```
GET /api/boxes/{id}
```

## 8. 趋势数据接口

### 8.1 获取当前趋势

```
GET /api/trends
```

**查询参数：**
| 参数 | 类型 | 说明 |
|------|------|------|
| symbol_id | int | 标的ID |
| period | string | 周期 |

**响应：**
```json
{
  "code": 0,
  "data": [
    {
      "id": 301,
      "symbol": {
        "id": 1,
        "code": "BTCUSDT"
      },
      "period": "1h",
      "trend_type": "bullish",
      "strength": 2,
      "ema_short": 67250.00,
      "ema_medium": 67100.00,
      "ema_long": 66950.00,
      "start_time": "2024-03-15T10:00:00Z",
      "status": "active"
    }
  ]
}
```

## 9. 系统配置接口

### 9.1 获取配置

```
GET /api/configs
```

**响应：**
```json
{
  "code": 0,
  "data": {
    "feishu_enabled": true,
    "strategy_box_enabled": true,
    "strategy_trend_enabled": true,
    "trading_enabled": true,
    "initial_capital": 100000.00
  }
}
```

### 9.2 更新配置

```
PUT /api/configs/{key}
```

**请求参数：**
```json
{
  "value": true
}
```

## 10. WebSocket 接口

### 10.1 连接地址

```
ws://localhost:8080/ws?token=<jwt_token>
```

### 10.2 订阅消息

**客户端发送：**
```json
{
  "action": "subscribe",
  "channels": ["kline:BTCUSDT:15m", "signal", "position"]
}
```

### 10.3 服务端推送

**K线更新：**
```json
{
  "type": "kline",
  "data": {
    "symbol_code": "BTCUSDT",
    "period": "15m",
    "kline": {
      "open": 67200.00,
      "high": 67350.00,
      "low": 67150.00,
      "close": 67245.00,
      "volume": 125.45
    }
  }
}
```

**信号通知：**
```json
{
  "type": "signal",
  "data": {
    "id": 1002,
    "symbol_code": "ETHUSDT",
    "signal_type": "trend_reversal",
    "direction": "long",
    "strength": 2,
    "price": 3452.00
  }
}
```

**持仓更新：**
```json
{
  "type": "position",
  "data": {
    "id": 501,
    "symbol_code": "BTCUSDT",
    "direction": "long",
    "entry_price": 67345.00,
    "current_price": 67500.00,
    "unrealized_pnl": 232.50,
    "unrealized_pnl_pct": 2.30
  }
}
```

## 11. 错误响应格式

```json
{
  "code": 1001,
  "message": "参数错误: symbol_id 不能为空",
  "timestamp": 1234567890
}
```
