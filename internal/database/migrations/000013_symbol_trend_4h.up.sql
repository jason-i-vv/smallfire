-- 添加4h趋势字段到symbols表
ALTER TABLE symbols ADD COLUMN IF NOT EXISTS trend_4h VARCHAR(20) DEFAULT 'sideways';
ALTER TABLE symbols ADD COLUMN IF NOT EXISTS trend_updated_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_symbols_trend_4h ON symbols(trend_4h) WHERE trend_4h IS NOT NULL;
