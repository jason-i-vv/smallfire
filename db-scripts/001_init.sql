-- 星火量化系统数据库初始化脚本
-- PostgreSQL 18.0

-- 创建数据库（如果不存在）
-- CREATE DATABASE starfire_quant;

-- 连接到数据库后执行以下脚本

-- ============================================
-- 1. 市场表
-- ============================================
CREATE TABLE IF NOT EXISTS markets (
    id              SERIAL PRIMARY KEY,
    market_code     VARCHAR(20) NOT NULL UNIQUE,
    market_name     VARCHAR(50) NOT NULL,
    market_type     VARCHAR(20) NOT NULL,
    is_enabled      BOOLEAN DEFAULT true,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_markets_code ON markets(market_code);

-- 插入默认市场数据
INSERT INTO markets (market_code, market_name, market_type) VALUES
    ('bybit', 'Bybit交易所', 'crypto'),
    ('a_stock', 'A股市场', 'stock'),
    ('us_stock', '美股市场', 'stock')
ON CONFLICT (market_code) DO NOTHING;

-- ============================================
-- 2. 交易标的表
-- ============================================
CREATE TABLE IF NOT EXISTS symbols (
    id                  SERIAL PRIMARY KEY,
    market_id           INTEGER REFERENCES markets(id),
    market_code         VARCHAR(20) NOT NULL,
    symbol_code         VARCHAR(30) NOT NULL,
    symbol_name         VARCHAR(100),
    symbol_type         VARCHAR(20),
    last_hot_at         TIMESTAMP,
    hot_score           DECIMAL(10, 2) DEFAULT 0,
    is_tracking         BOOLEAN DEFAULT true,
    max_klines_count    INTEGER DEFAULT 1000,
    created_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(market_code, symbol_code)
);

CREATE INDEX IF NOT EXISTS idx_symbols_market ON symbols(market_id);
CREATE INDEX IF NOT EXISTS idx_symbols_market_code ON symbols(market_code);
CREATE INDEX IF NOT EXISTS idx_symbols_hot ON symbols(hot_score DESC, last_hot_at);
CREATE INDEX IF NOT EXISTS idx_symbols_tracking ON symbols(is_tracking) WHERE is_tracking = true;

-- ============================================
-- 3. K线数据表
-- ============================================
CREATE TABLE IF NOT EXISTS klines (
    id              BIGSERIAL PRIMARY KEY,
    symbol_id       INTEGER REFERENCES symbols(id),
    period          VARCHAR(10) NOT NULL,
    open_time       TIMESTAMP NOT NULL,
    close_time      TIMESTAMP NOT NULL,
    open_price      DECIMAL(20, 8) NOT NULL,
    high_price      DECIMAL(20, 8) NOT NULL,
    low_price       DECIMAL(20, 8) NOT NULL,
    close_price     DECIMAL(20, 8) NOT NULL,
    volume          DECIMAL(20, 8) NOT NULL,
    quote_volume    DECIMAL(20, 8),
    trades_count    INTEGER DEFAULT 0,
    is_closed       BOOLEAN DEFAULT true,
    ema_short       DECIMAL(20, 8),
    ema_medium      DECIMAL(20, 8),
    ema_long        DECIMAL(20, 8),
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(symbol_id, period, open_time)
);

CREATE INDEX IF NOT EXISTS idx_klines_symbol_period ON klines(symbol_id, period);
CREATE INDEX IF NOT EXISTS idx_klines_open_time ON klines(open_time DESC);
CREATE INDEX IF NOT EXISTS idx_klines_symbol_period_time ON klines(symbol_id, period, open_time DESC);

-- ============================================
-- 4. 箱体数据表
-- ============================================
CREATE TABLE IF NOT EXISTS price_boxes (
    id                  SERIAL PRIMARY KEY,
    symbol_id           INTEGER REFERENCES symbols(id),
    box_type            VARCHAR(10) NOT NULL,
    status              VARCHAR(20) NOT NULL DEFAULT 'active',
    high_price          DECIMAL(20, 8) NOT NULL,
    low_price           DECIMAL(20, 8) NOT NULL,
    width_price         DECIMAL(20, 8) NOT NULL,
    width_percent       DECIMAL(8, 4),
    klines_count        INTEGER NOT NULL,
    start_time          TIMESTAMP NOT NULL,
    end_time            TIMESTAMP,
    breakout_price      DECIMAL(20, 8),
    breakout_direction  VARCHAR(10),
    breakout_time       TIMESTAMP,
    breakout_kline_id   BIGINT,
    subscriber_count    INTEGER DEFAULT 1,
    created_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_boxes_symbol ON price_boxes(symbol_id);
CREATE INDEX IF NOT EXISTS idx_boxes_status ON price_boxes(status) WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_boxes_start ON price_boxes(start_time DESC);

-- ============================================
-- 5. 趋势数据表
-- ============================================
CREATE TABLE IF NOT EXISTS trends (
    id              SERIAL PRIMARY KEY,
    symbol_id       INTEGER REFERENCES symbols(id),
    period          VARCHAR(10) NOT NULL,
    trend_type      VARCHAR(20) NOT NULL,
    strength        INTEGER NOT NULL DEFAULT 0,
    ema_short       DECIMAL(20, 8),
    ema_medium      DECIMAL(20, 8),
    ema_long        DECIMAL(20, 8),
    start_time      TIMESTAMP NOT NULL,
    end_time        TIMESTAMP,
    status          VARCHAR(20) DEFAULT 'active',
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_trends_symbol ON trends(symbol_id);
CREATE INDEX IF NOT EXISTS idx_trends_active ON trends(symbol_id, period) WHERE status = 'active';

-- ============================================
-- 6. 关键价位表
-- ============================================
CREATE TABLE IF NOT EXISTS key_levels (
    id              SERIAL PRIMARY KEY,
    symbol_id       INTEGER REFERENCES symbols(id),
    level_type      VARCHAR(20) NOT NULL,
    level_subtype   VARCHAR(20) NOT NULL,
    price           DECIMAL(20, 8) NOT NULL,
    period          VARCHAR(10) NOT NULL,
    broken          BOOLEAN DEFAULT false,
    broken_at       TIMESTAMP,
    broken_price    DECIMAL(20, 8),
    broken_direction VARCHAR(10),
    klines_count    INTEGER,
    valid_until     TIMESTAMP,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_levels_symbol ON key_levels(symbol_id);
CREATE INDEX IF NOT EXISTS idx_levels_broken ON key_levels(broken) WHERE broken = false;

-- ============================================
-- 7. 交易信号表
-- ============================================
CREATE TABLE IF NOT EXISTS signals (
    id                  SERIAL PRIMARY KEY,
    symbol_id           INTEGER REFERENCES symbols(id),
    signal_type         VARCHAR(30) NOT NULL,
    source_type         VARCHAR(20) NOT NULL,
    direction           VARCHAR(10) NOT NULL,
    strength            INTEGER NOT NULL DEFAULT 1,
    price               DECIMAL(20, 8) NOT NULL,
    target_price        DECIMAL(20, 8),
    stop_loss_price     DECIMAL(20, 8),
    period              VARCHAR(10),
    signal_data         JSONB,
    status              VARCHAR(20) DEFAULT 'pending',
    confirmed_at        TIMESTAMP,
    expired_at          TIMESTAMP,
    triggered_at        TIMESTAMP,
    notification_sent   BOOLEAN DEFAULT false,
    created_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_signals_symbol ON signals(symbol_id);
CREATE INDEX IF NOT EXISTS idx_signals_type ON signals(signal_type);
CREATE INDEX IF NOT EXISTS idx_signals_status ON signals(status);
CREATE INDEX IF NOT EXISTS idx_signals_created ON signals(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_signals_pending ON signals(status) WHERE status = 'pending';

-- ============================================
-- 8. 交易跟踪表
-- ============================================
CREATE TABLE IF NOT EXISTS trade_tracks (
    id                      SERIAL PRIMARY KEY,
    signal_id               INTEGER REFERENCES signals(id),
    symbol_id               INTEGER REFERENCES symbols(id),
    direction               VARCHAR(10) NOT NULL,
    entry_price             DECIMAL(20, 8),
    entry_time              TIMESTAMP,
    quantity                DECIMAL(20, 8),
    position_value          DECIMAL(20, 8),
    stop_loss_price         DECIMAL(20, 8),
    stop_loss_percent       DECIMAL(6, 4),
    take_profit_price       DECIMAL(20, 8),
    take_profit_percent     DECIMAL(6, 4),
    trailing_stop_enabled    BOOLEAN DEFAULT false,
    trailing_stop_active     BOOLEAN DEFAULT false,
    trailing_stop_price     DECIMAL(20, 8),
    trailing_activation_pct  DECIMAL(6, 4),
    exit_price              DECIMAL(20, 8),
    exit_time               TIMESTAMP,
    exit_reason             VARCHAR(30),
    pnl                     DECIMAL(20, 8),
    pnl_percent             DECIMAL(8, 4),
    fees                    DECIMAL(10, 4) DEFAULT 0,
    status                  VARCHAR(20) DEFAULT 'open',
    current_price           DECIMAL(20, 8),
    unrealized_pnl          DECIMAL(20, 8),
    unrealized_pnl_pct      DECIMAL(8, 4),
    subscriber_count        INTEGER DEFAULT 1,
    created_at              TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at              TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_tracks_signal ON trade_tracks(signal_id);
CREATE INDEX IF NOT EXISTS idx_tracks_symbol ON trade_tracks(symbol_id);
CREATE INDEX IF NOT EXISTS idx_tracks_status ON trade_tracks(status);
CREATE INDEX IF NOT EXISTS idx_tracks_entry ON trade_tracks(entry_time DESC);

-- ============================================
-- 9. 实时监测表
-- ============================================
CREATE TABLE IF NOT EXISTS monitorings (
    id                  SERIAL PRIMARY KEY,
    symbol_id           INTEGER REFERENCES symbols(id),
    symbol_code         VARCHAR(30),
    monitor_type        VARCHAR(20) NOT NULL,
    target_price        DECIMAL(20, 8),
    condition_type      VARCHAR(20) NOT NULL,
    reference_price     DECIMAL(20, 8),
    subscriber_count    INTEGER DEFAULT 1,
    is_active           BOOLEAN DEFAULT true,
    triggered_at        TIMESTAMP,
    created_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at          TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_monitorings_symbol ON monitorings(symbol_id);
CREATE INDEX IF NOT EXISTS idx_monitorings_active ON monitorings(is_active) WHERE is_active = true;

-- ============================================
-- 10. 用户表
-- ============================================
CREATE TABLE IF NOT EXISTS users (
    id              SERIAL PRIMARY KEY,
    username        VARCHAR(50) NOT NULL UNIQUE,
    password_hash   VARCHAR(255) NOT NULL,
    nickname        VARCHAR(50),
    role            VARCHAR(20) DEFAULT 'user',
    is_active       BOOLEAN DEFAULT true,
    last_login_at   TIMESTAMP,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 创建默认管理员用户 (密码: admin123)
-- 实际使用时请更换为安全的密码和哈希值
INSERT INTO users (username, password_hash, nickname, role) VALUES
    ('admin', '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', '管理员', 'admin')
ON CONFLICT (username) DO NOTHING;

-- ============================================
-- 11. 系统配置表
-- ============================================
CREATE TABLE IF NOT EXISTS configs (
    id              SERIAL PRIMARY KEY,
    config_key      VARCHAR(100) NOT NULL UNIQUE,
    config_value    JSONB NOT NULL,
    description     VARCHAR(255),
    is_public       BOOLEAN DEFAULT false,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 插入默认配置
INSERT INTO configs (config_key, config_value, description, is_public) VALUES
    ('feishu', '{"enabled": true, "webhook_url": "", "send_summary": true, "summary_interval": 6}', '飞书通知配置', true),
    ('strategies.box', '{"enabled": true, "min_klines": 5, "max_klines": 50, "width_threshold": 0.02}', '箱体策略配置', true),
    ('strategies.trend', '{"enabled": true, "ema_periods": [30, 60, 90]}', '趋势策略配置', true),
    ('strategies.key_level', '{"enabled": true, "lookback_klines": 100}', '关键价位策略配置', true),
    ('strategies.volume_price', '{"enabled": true, "volatility_multiplier": 2, "volume_multiplier": 2}', '量价异常策略配置', true),
    ('trading', '{"enabled": true, "initial_capital": 100000, "position_size": 0.1, "stop_loss_percent": 0.02, "take_profit_percent": 0.05}', '交易配置', true)
ON CONFLICT (config_key) DO NOTHING;

-- ============================================
-- 12. 通知记录表
-- ============================================
CREATE TABLE IF NOT EXISTS notifications (
    id              SERIAL PRIMARY KEY,
    signal_id       INTEGER REFERENCES signals(id),
    channel         VARCHAR(20) NOT NULL,
    content         JSONB,
    status          VARCHAR(20) DEFAULT 'pending',
    sent_at         TIMESTAMP,
    error_message   TEXT,
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_notifications_signal ON notifications(signal_id);
CREATE INDEX IF NOT EXISTS idx_notifications_status ON notifications(status);

-- ============================================
-- 触发器：自动更新 updated_at
-- ============================================
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- 为需要自动更新的表创建触发器
CREATE TRIGGER update_markets_updated_at BEFORE UPDATE ON markets
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_symbols_updated_at BEFORE UPDATE ON symbols
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_price_boxes_updated_at BEFORE UPDATE ON price_boxes
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_trends_updated_at BEFORE UPDATE ON trends
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_key_levels_updated_at BEFORE UPDATE ON key_levels
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_signals_updated_at BEFORE UPDATE ON signals
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_trade_tracks_updated_at BEFORE UPDATE ON trade_tracks
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_monitorings_updated_at BEFORE UPDATE ON monitorings
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_configs_updated_at BEFORE UPDATE ON configs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ============================================
-- 注释说明
-- ============================================
COMMENT ON TABLE markets IS '市场表';
COMMENT ON TABLE symbols IS '交易标的表';
COMMENT ON TABLE klines IS 'K线数据表';
COMMENT ON TABLE price_boxes IS '箱体数据表';
COMMENT ON TABLE trends IS '趋势数据表';
COMMENT ON TABLE key_levels IS '关键价位表';
COMMENT ON TABLE signals IS '交易信号表';
COMMENT ON TABLE trade_tracks IS '交易跟踪表';
COMMENT ON TABLE monitorings IS '实时监测表';
COMMENT ON TABLE users IS '用户表';
COMMENT ON TABLE configs IS '系统配置表';
COMMENT ON TABLE notifications IS '通知记录表';
