-- 新关键价位表：按币对+周期存储，upsert 覆盖
CREATE TABLE IF NOT EXISTS key_levels_v2 (
    id SERIAL PRIMARY KEY,
    symbol_id INT NOT NULL,
    period VARCHAR(10) NOT NULL,
    resistances JSONB,  -- [{"price": 2215.00, "strength": "strong", "reason": "..."}, ...]
    supports JSONB,     -- [{"price": 2192.74, "strength": "mid", "reason": "..."}, ...]
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(symbol_id, period)
);

CREATE INDEX IF NOT EXISTS idx_key_levels_v2_symbol_period ON key_levels_v2(symbol_id, period);
