-- A股涨跌停每日统计表
CREATE TABLE IF NOT EXISTS a_stock_limit_stats (
    id               SERIAL PRIMARY KEY,
    trade_date       DATE NOT NULL UNIQUE,
    up_limit_count   INT NOT NULL DEFAULT 0,
    down_limit_count INT NOT NULL DEFAULT 0,
    created_at       TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_limit_stats_trade_date ON a_stock_limit_stats(trade_date DESC);
