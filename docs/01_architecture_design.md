# 星火量化系统 - 整体架构设计

## 1. 系统架构概述

```
┌─────────────────────────────────────────────────────────────────┐
│                         控制台 (Vue Frontend)                     │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────────┐ │
│  │ 登录模块 │ │ 信号展示 │ │ 交易统计 │ │ 策略配置 │ │ 实时K线图表 │ │
│  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────────┘ │
└─────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Web API (Go Gin/Echo)                       │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌───────────┐  │
│  │ 认证API │ │ 信号API │ │ 交易API │ │ 行情API │ │ 统计API  │  │
│  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └───────────┘  │
└─────────────────────────────────────────────────────────────────┘
                                │
        ┌───────────────────────┼───────────────────────┐
        ▼                       ▼                       ▼
┌───────────────┐     ┌───────────────┐     ┌───────────────┐
│  行情抓取模块  │     │  策略分析模块  │     │  消息通知模块  │
│  - bybit      │     │  - 箱体策略   │     │  - 飞书通知   │
│  - A股        │     │  - 趋势策略   │     └───────────────┘
│  - 美股       │     │  - 阻力支撑   │
└───────────────┘     │  - 量价异常   │
        │             └───────────────┘
        ▼                     │
┌───────────────┐             │
│  实时监测模块  │◄────────────┘
│  - 价格订阅   │
│  - 事件触发   │
└───────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────────────┐
│                       PostgreSQL 18.0                            │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌───────────┐   │
│  │ K线数据  │ │ 交易信号 │ │ 交易跟踪 │ │ 箱体数据  │ │ 系统配置  │   │
│  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └───────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

## 2. 数据库表结构设计

### 2.1 市场与标的表 (markets)

```sql
CREATE TABLE markets (
    id              SERIAL PRIMARY KEY,
    market_code     VARCHAR(20) NOT NULL UNIQUE,  -- 'bybit', 'a_stock', 'us_stock'
    market_name     VARCHAR(50) NOT NULL,
    market_type     VARCHAR(20) NOT NULL,          -- 'crypto', 'stock', 'futures'
    is_enabled      BOOLEAN DEFAULT true,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_markets_code ON markets(market_code);
```

### 2.2 交易标的表 (symbols)

```sql
CREATE TABLE symbols (
    id                  SERIAL PRIMARY KEY,
    market_id           INTEGER REFERENCES markets(id),
    symbol_code         VARCHAR(30) NOT NULL,      -- 'BTCUSDT', '600519'
    symbol_name         VARCHAR(100),
    symbol_type         VARCHAR(20),               -- 'spot', 'futures', 'option'
    last_hot_at         TIMESTAMP,                  -- 最后一次进入热门的时间
    hot_score           DECIMAL(10, 2) DEFAULT 0,   -- 热度评分
    is_tracking         BOOLEAN DEFAULT true,       -- 是否正在跟踪
    max_klines_count    INTEGER DEFAULT 1000,       -- 最大K线缓存数量
    created_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(market_id, symbol_code)
);

CREATE INDEX idx_symbols_market ON symbols(market_id);
CREATE INDEX idx_symbols_hot ON symbols(hot_score DESC, last_hot_at);
CREATE INDEX idx_symbols_tracking ON symbols(is_tracking) WHERE is_tracking = true;
```

### 2.3 K线数据表 (klines)

```sql
CREATE TABLE klines (
    id              BIGSERIAL PRIMARY KEY,
    symbol_id       INTEGER REFERENCES symbols(id),
    period          VARCHAR(10) NOT NULL,           -- '1m', '5m', '15m', '1h', '4h', '1d'
    open_time       TIMESTAMP NOT NULL,
    close_time      TIMESTAMP NOT NULL,
    open_price      DECIMAL(20, 8) NOT NULL,
    high_price      DECIMAL(20, 8) NOT NULL,
    low_price       DECIMAL(20, 8) NOT NULL,
    close_price     DECIMAL(20, 8) NOT NULL,
    volume          DECIMAL(20, 8) NOT NULL,
    quote_volume    DECIMAL(20, 8),                 -- 成交额
    trades_count    INTEGER DEFAULT 0,              -- 成交笔数
    is_closed       BOOLEAN DEFAULT true,           -- K线是否已关闭
    ema_short       DECIMAL(20, 8),                 -- EMA短期 (30)
    ema_medium      DECIMAL(20, 8),                 -- EMA中期 (60)
    ema_long        DECIMAL(20, 8),                 -- EMA长期 (90)
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(symbol_id, period, open_time)
);

CREATE INDEX idx_klines_symbol_period ON klines(symbol_id, period);
CREATE INDEX idx_klines_open_time ON klines(open_time DESC);
CREATE INDEX idx_klines_symbol_period_time ON klines(symbol_id, period, open_time DESC);
```

### 2.4 箱体数据表 (price_boxes)

```sql
CREATE TABLE price_boxes (
    id                  SERIAL PRIMARY KEY,
    symbol_id           INTEGER REFERENCES symbols(id),
    box_type            VARCHAR(10) NOT NULL,       -- 'consolidation', 'breakout', 'breakdown'
    status              VARCHAR(20) NOT NULL DEFAULT 'active',  -- 'active', 'closed'
    high_price          DECIMAL(20, 8) NOT NULL,     -- 箱体上沿
    low_price           DECIMAL(20, 8) NOT NULL,     -- 箱体下沿
    width_price         DECIMAL(20, 8) NOT NULL,     -- 箱体宽度
    width_percent       DECIMAL(8, 4),               -- 箱体宽度百分比
    klines_count        INTEGER NOT NULL,            -- 形成箱体的K线数量
    start_time          TIMESTAMP NOT NULL,          -- 箱体开始时间
    end_time            TIMESTAMP,                   -- 箱体结束时间
    breakout_price      DECIMAL(20, 8),              -- 突破价格
    breakout_direction  VARCHAR(10),                 -- 'up', 'down'
    breakout_time       TIMESTAMP,                   -- 突破时间
    breakout_kline_id   BIGINT,                      -- 触发突破的K线ID
    subscriber_count    INTEGER DEFAULT 1,           -- 订阅数量
    created_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_boxes_symbol ON price_boxes(symbol_id);
CREATE INDEX idx_boxes_status ON price_boxes(status) WHERE status = 'active';
CREATE INDEX idx_boxes_start ON price_boxes(start_time DESC);
```

### 2.5 趋势数据表 (trends)

```sql
CREATE TABLE trends (
    id              SERIAL PRIMARY KEY,
    symbol_id       INTEGER REFERENCES symbols(id),
    period          VARCHAR(10) NOT NULL,           -- '15m', '1h', '1d'
    trend_type      VARCHAR(20) NOT NULL,           -- 'bullish', 'bearish', 'sideways'
    strength        INTEGER NOT NULL DEFAULT 0,      -- 强度 1-3
    ema_short       DECIMAL(20, 8),
    ema_medium      DECIMAL(20, 8),
    ema_long        DECIMAL(20, 8),
    start_time      TIMESTAMP NOT NULL,             -- 趋势开始时间
    end_time        TIMESTAMP,                      -- 趋势结束时间
    status          VARCHAR(20) DEFAULT 'active',   -- 'active', 'ended'
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(symbol_id, period, status) WHERE status = 'active'
);

CREATE INDEX idx_trends_symbol ON trends(symbol_id);
CREATE INDEX idx_trends_status ON trends(status) WHERE status = 'active';
```

### 2.6 关键价位表 (key_levels)

```sql
CREATE TABLE key_levels (
    id              SERIAL PRIMARY KEY,
    symbol_id       INTEGER REFERENCES symbols(id),
    level_type      VARCHAR(20) NOT NULL,           -- 'resistance', 'support'
    level_subtype   VARCHAR(20) NOT NULL,           -- 'current_high', 'prev_high', 'current_low', 'prev_low'
    price           DECIMAL(20, 8) NOT NULL,
    period          VARCHAR(10) NOT NULL,           -- '15m', '1h', '1d'
    broken          BOOLEAN DEFAULT false,          -- 是否被突破
    broken_at       TIMESTAMP,                      -- 被突破时间
    broken_price    DECIMAL(20, 8),                 -- 突破时的价格
    broken_direction VARCHAR(10),                   -- 'up', 'down'
    klines_count    INTEGER,                        -- 触及次数
    valid_until     TIMESTAMP,                      -- 有效期
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_levels_symbol ON key_levels(symbol_id);
CREATE INDEX idx_levels_broken ON key_levels(broken) WHERE broken = false;
```

### 2.7 交易信号表 (signals)

```sql
CREATE TABLE signals (
    id                  SERIAL PRIMARY KEY,
    symbol_id           INTEGER REFERENCES symbols(id),
    signal_type         VARCHAR(30) NOT NULL,       -- 'box_breakout', 'box_breakdown', 'trend_reversal',
                                                    -- 'resistance_break', 'support_break',
                                                    -- 'volume_surge', 'price_surge'
    source_type         VARCHAR(20) NOT NULL,        -- 'box', 'trend', 'key_level', 'volume'
    direction           VARCHAR(10) NOT NULL,       -- 'long', 'short'
    strength            INTEGER NOT NULL DEFAULT 1,  -- 强度 1-3
    price               DECIMAL(20, 8) NOT NULL,    -- 信号产生时的价格
    target_price        DECIMAL(20, 8),             -- 目标价格
    stop_loss_price     DECIMAL(20, 8),             -- 止损价格
    period              VARCHAR(10),               -- K线周期
    signal_data         JSONB,                       -- 附加数据
    status              VARCHAR(20) DEFAULT 'pending', -- 'pending', 'confirmed', 'expired', 'triggered'
    confirmed_at        TIMESTAMP,
    expired_at          TIMESTAMP,
    triggered_at        TIMESTAMP,
    notification_sent   BOOLEAN DEFAULT false,
    created_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_signals_symbol ON signals(symbol_id);
CREATE INDEX idx_signals_type ON signals(signal_type);
CREATE INDEX idx_signals_status ON signals(status);
CREATE INDEX idx_signals_created ON signals(created_at DESC);
CREATE INDEX idx_signals_pending ON signals(status) WHERE status = 'pending';
```

### 2.8 交易跟踪表 (trade_tracks)

```sql
CREATE TABLE trade_tracks (
    id                  SERIAL PRIMARY KEY,
    signal_id           INTEGER REFERENCES signals(id),
    symbol_id           INTEGER REFERENCES symbols(id),
    direction           VARCHAR(10) NOT NULL,       -- 'long', 'short'
    entry_price         DECIMAL(20, 8),             -- 入场价格
    entry_time          TIMESTAMP,                  -- 入场时间
    quantity            DECIMAL(20, 8),             -- 持仓数量
    position_value      DECIMAL(20, 8),             -- 持仓价值

    stop_loss_price     DECIMAL(20, 8),             -- 止损价格
    stop_loss_percent   DECIMAL(6, 4),              -- 止损百分比
    take_profit_price   DECIMAL(20, 8),            -- 止盈价格
    take_profit_percent  DECIMAL(6, 4),             -- 止盈百分比

    exit_price          DECIMAL(20, 8),             -- 出场价格
    exit_time           TIMESTAMP,                  -- 出场时间
    exit_reason         VARCHAR(30),                -- 'stop_loss', 'take_profit', 'manual'

    pnl                 DECIMAL(20, 8),             -- 盈亏金额
    pnl_percent         DECIMAL(8, 4),               -- 盈亏百分比
    fees                DECIMAL(10, 4) DEFAULT 0,  -- 手续费

    status              VARCHAR(20) DEFAULT 'open', -- 'open', 'closed'
    current_price       DECIMAL(20, 8),             -- 当前价格
    unrealized_pnl      DECIMAL(20, 8),             -- 未实现盈亏
    unrealized_pnl_pct  DECIMAL(8, 4),              -- 未实现盈亏百分比

    subscriber_count    INTEGER DEFAULT 1,           -- 订阅数量
    created_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_tracks_signal ON trade_tracks(signal_id);
CREATE INDEX idx_tracks_symbol ON trade_tracks(symbol_id);
CREATE INDEX idx_tracks_status ON trade_tracks(status);
CREATE INDEX idx_tracks_entry ON trade_tracks(entry_time DESC);
```

### 2.9 实时监测表 (monitorings)

```sql
CREATE TABLE monitorings (
    id                  SERIAL PRIMARY KEY,
    symbol_id           INTEGER REFERENCES symbols(id),
    monitor_type        VARCHAR(20) NOT NULL,       -- 'price', 'box', 'trend'
    target_price        DECIMAL(20, 8),              -- 目标价格
    condition_type      VARCHAR(20) NOT NULL,       -- 'greater', 'less', 'equal', 'cross_up', 'cross_down'
    reference_price     DECIMAL(20, 8),             -- 参考价格
    subscriber_count    INTEGER DEFAULT 1,           -- 订阅数量
    is_active           BOOLEAN DEFAULT true,
    triggered_at        TIMESTAMP,
    created_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_monitorings_symbol ON monitorings(symbol_id);
CREATE INDEX idx_monitorings_active ON monitorings(is_active) WHERE is_active = true;
```

### 2.10 用户表 (users)

```sql
CREATE TABLE users (
    id              SERIAL PRIMARY KEY,
    username        VARCHAR(50) NOT NULL UNIQUE,
    password_hash   VARCHAR(255) NOT NULL,
    nickname        VARCHAR(50),
    role            VARCHAR(20) DEFAULT 'user',     -- 'admin', 'user'
    is_active       BOOLEAN DEFAULT true,
    last_login_at   TIMESTAMP,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### 2.11 系统配置表 (configs)

```sql
CREATE TABLE configs (
    id              SERIAL PRIMARY KEY,
    config_key      VARCHAR(100) NOT NULL UNIQUE,
    config_value    JSONB NOT NULL,
    description     VARCHAR(255),
    is_public       BOOLEAN DEFAULT false,          -- 是否公开给前端
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### 2.12 通知记录表 (notifications)

```sql
CREATE TABLE notifications (
    id              SERIAL PRIMARY KEY,
    signal_id       INTEGER REFERENCES signals(id),
    channel         VARCHAR(20) NOT NULL,           -- 'feishu', 'email', 'sms'
    content         JSONB,
    status          VARCHAR(20) DEFAULT 'pending',  -- 'pending', 'sent', 'failed'
    sent_at         TIMESTAMP,
    error_message   TEXT,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_notifications_signal ON notifications(signal_id);
CREATE INDEX idx_notifications_status ON notifications(status);
```

## 3. 配置文件结构 (config.yml)

```yaml
# 系统基础配置
app:
  name: "星火量化"
  mode: "release"                    # debug, release, test
  host: "0.0.0.0"
  port: 8080
  timezone: "Asia/Shanghai"           # UTC+8

# 数据库配置
database:
  host: "localhost"
  port: 5432
  user: "postgres"
  password: "${DB_PASSWORD}"
  dbname: "starfire_quant"
  sslmode: "disable"
  max_open_conns: 25
  max_idle_conns: 5
  conn_max_lifetime: 5m

# 日志配置
log:
  level: "info"                      # debug, info, warn, error
  format: "json"                     # json, text
  output: "stdout"                   # stdout, file
  file_path: "./logs/starfire.log"
  max_size: 100
  max_backups: 30
  max_age: 7

# 飞书通知配置
feishu:
  enabled: true
  webhook_url: "https://open.feishu.cn/open-apis/bot/v2/hook/xxx"
  send_summary: true
  summary_interval: 6                # 小时
  summary_time: "09:00,15:00,21:00"  # UTC+8时间

# 市场配置
markets:
  bybit:
    enabled: true
    api_key: "${BYBIT_API_KEY}"
    api_secret: "${BYBIT_API_SECRET}"
    testnet: false
    symbols_limit: 200
    hot_days: 30
    periods: ["15m", "1h"]
    fetch_interval: 60               # 秒

  a_stock:
    enabled: true
    source: "eastmoney"              # 数据源
    symbols_limit: 200
    hot_days: 30
    periods: ["1d"]
    fetch_interval: 300             # 秒

  us_stock:
    enabled: true
    source: "yahoo"
    symbols_limit: 200
    hot_days: 30
    periods: ["1d"]
    fetch_interval: 300             # 秒

# 策略配置
strategies:
  # 箱体策略
  box:
    enabled: true
    min_klines: 5                   # 最少K线数形成箱体
    max_klines: 50                  # 最大K线数
    width_threshold: 0.02            # 箱体宽度阈值 (2%)
    breakout_buffer: 0.001          # 突破缓冲 (0.1%)
    check_interval: 60              # 检查间隔秒

  # 趋势策略
  trend:
    enabled: true
    ema_periods: [30, 60, 90]       # EMA周期
    strength_thresholds:
      level1: 1                     # 短期
      level2: 2                     # 中期
      level3: 3                     # 长期
    check_interval: 300

  # 阻力支撑策略
  key_level:
    enabled: true
    lookback_klines: 100            # 回溯K线数
    level_distance: 0.01            # 价位间距阈值
    check_interval: 60

  # 量价异常策略
  volume_price_anomaly:
    enabled: true
    volatility_multiplier: 2        # 波动倍数
    volume_multiplier: 2            # 成交量倍数
    lookback_klines: 20             # 回溯K线数
    check_interval: 60

# 交易跟踪配置
trading:
  enabled: true
  initial_capital: 100000           # 初始资金
  position_size: 0.1                # 单笔仓位比例
  stop_loss_percent: 0.02           # 止损比例 (2%)
  take_profit_percent: 0.05         # 止盈比例 (5%)
  trailing_stop: true               # 启用移动止损
  trailing_distance: 0.015         # 移动止损距离 (1.5%)
  max_trades_per_day: 10           # 每日最大交易数

# 实时监测配置
monitoring:
  price_check_interval: 1          # 价格检查间隔 (秒)
  max_concurrent_monitors: 1000     # 最大并发监测数
  cleanup_interval: 300            # 清理间隔 (秒)

# 前端配置
frontend:
  ws_url: "ws://localhost:8080/ws"
  refresh_interval: 5000           # 刷新间隔 (毫秒)
  kline_cache_count: 100           # K线缓存数量
```
