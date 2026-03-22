-- 创建数据库表结构
-- 市场表
CREATE TABLE IF NOT EXISTS markets (
    id SERIAL PRIMARY KEY,
    market_code VARCHAR(20) NOT NULL UNIQUE,
    market_name VARCHAR(100) NOT NULL,
    market_type VARCHAR(20) NOT NULL,
    is_enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- 标的表
CREATE TABLE IF NOT EXISTS symbols (
    id SERIAL PRIMARY KEY,
    market_id INTEGER NOT NULL,
    market_code VARCHAR(20) NOT NULL,
    symbol_code VARCHAR(50) NOT NULL,
    symbol_name VARCHAR(200),
    symbol_type VARCHAR(20),
    last_hot_at TIMESTAMPTZ,
    hot_score DOUBLE PRECISION DEFAULT 0,
    is_tracking BOOLEAN DEFAULT false,
    max_klines_count INTEGER DEFAULT 1000,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (market_id) REFERENCES markets(id),
    UNIQUE(market_code, symbol_code)
);

-- K线表
CREATE TABLE IF NOT EXISTS klines (
    id SERIAL PRIMARY KEY,
    symbol_id INTEGER NOT NULL,
    period VARCHAR(10) NOT NULL,
    open_time TIMESTAMPTZ NOT NULL,
    close_time TIMESTAMPTZ NOT NULL,
    open_price DOUBLE PRECISION NOT NULL,
    high_price DOUBLE PRECISION NOT NULL,
    low_price DOUBLE PRECISION NOT NULL,
    close_price DOUBLE PRECISION NOT NULL,
    volume DOUBLE PRECISION NOT NULL,
    quote_volume DOUBLE PRECISION,
    trades_count INTEGER,
    is_closed BOOLEAN DEFAULT true,
    ema_short DOUBLE PRECISION,
    ema_medium DOUBLE PRECISION,
    ema_long DOUBLE PRECISION,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (symbol_id) REFERENCES symbols(id),
    UNIQUE(symbol_id, period, open_time)
);

-- 索引
CREATE INDEX IF NOT EXISTS idx_klines_symbol_period ON klines(symbol_id, period);
CREATE INDEX IF NOT EXISTS idx_klines_open_time ON klines(open_time DESC);
CREATE INDEX IF NOT EXISTS idx_symbols_market_code ON symbols(market_code);
CREATE INDEX IF NOT EXISTS idx_symbols_is_tracking ON symbols(is_tracking);

-- 初始化市场数据
INSERT INTO markets (market_code, market_name, market_type, is_enabled, created_at, updated_at)
VALUES
    ('bybit', 'Bybit', 'crypto', true, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('a_stock', 'A股', 'stock', false, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
    ('us_stock', '美股', 'stock', false, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (market_code) DO NOTHING;

-- 更新时间戳函数
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- 创建更新触发器
CREATE TRIGGER trigger_update_markets
    BEFORE UPDATE ON markets
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER trigger_update_symbols
    BEFORE UPDATE ON symbols
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER trigger_update_klines
    BEFORE UPDATE ON klines
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();
