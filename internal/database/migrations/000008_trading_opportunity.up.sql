-- 信号评分相关字段
ALTER TABLE signals ADD COLUMN IF NOT EXISTS score INTEGER DEFAULT 0;
ALTER TABLE signals ADD COLUMN IF NOT EXISTS score_details JSONB;
ALTER TABLE signals ADD COLUMN IF NOT EXISTS valid_until TIMESTAMP;
ALTER TABLE signals ADD COLUMN IF NOT EXISTS confluence_info JSONB;

-- 新增信号状态：absorbed（被交易机会吸收）
-- 注意：status 是 VARCHAR(20)，直接使用新值即可，无需 ALTER

-- 交易机会表
CREATE TABLE IF NOT EXISTS trading_opportunities (
    id SERIAL PRIMARY KEY,
    symbol_id INTEGER NOT NULL REFERENCES symbols(id),
    symbol_code VARCHAR(20) NOT NULL,
    direction VARCHAR(10) NOT NULL,              -- long / short
    score INTEGER NOT NULL DEFAULT 0,            -- 综合评分 0-100
    score_details JSONB,                          -- 评分维度明细
    signal_count INTEGER NOT NULL DEFAULT 0,      -- 聚合的信号数量
    confluence_directions TEXT[],                  -- 共识方向列表
    confluence_ratio NUMERIC(5,2),                -- 共识比例 (如 0.67 = 2/3)
    suggested_entry DECIMAL(20,8),                -- 建议入场价
    suggested_stop_loss DECIMAL(20,8),            -- 建议止损价
    suggested_take_profit DECIMAL(20,8),          -- 建议止盈价
    ai_adjustment INTEGER DEFAULT 0,              -- AI 调整分 (-15 ~ +15)
    ai_judgment JSONB,                            -- AI 判定结果
    status VARCHAR(20) NOT NULL DEFAULT 'active', -- active / expired / triggered / cancelled
    period VARCHAR(10),                           -- 主周期
    first_signal_at TIMESTAMP,                    -- 最早信号时间
    last_signal_at TIMESTAMP,                     -- 最新信号时间
    expired_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_opportunities_symbol ON trading_opportunities(symbol_id);
CREATE INDEX IF NOT EXISTS idx_opportunities_status ON trading_opportunities(status);
CREATE INDEX IF NOT EXISTS idx_opportunities_score ON trading_opportunities(score DESC);
CREATE INDEX IF NOT EXISTS idx_opportunities_created ON trading_opportunities(created_at DESC);

-- 信号类型统计表
CREATE TABLE IF NOT EXISTS signal_type_stats (
    id SERIAL PRIMARY KEY,
    signal_type VARCHAR(50) NOT NULL,
    direction VARCHAR(10) NOT NULL,
    period VARCHAR(10) NOT NULL,
    symbol_id INTEGER,
    total_trades INTEGER DEFAULT 0,
    win_count INTEGER DEFAULT 0,
    loss_count INTEGER DEFAULT 0,
    win_rate NUMERIC(6,4) DEFAULT 0,
    avg_return NUMERIC(10,6) DEFAULT 0,
    profit_factor NUMERIC(10,4) DEFAULT 0,
    optimal_stop_loss NUMERIC(8,6),
    optimal_take_profit NUMERIC(8,6),
    stats_window_days INTEGER DEFAULT 30,
    last_trade_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(signal_type, direction, period, symbol_id)
);

CREATE INDEX IF NOT EXISTS idx_signal_type_stats_lookup ON signal_type_stats(signal_type, direction, period);
