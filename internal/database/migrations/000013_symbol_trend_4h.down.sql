DROP INDEX IF EXISTS idx_symbols_trend_4h;
ALTER TABLE symbols DROP COLUMN IF EXISTS trend_updated_at;
ALTER TABLE symbols DROP COLUMN IF EXISTS trend_4h;
