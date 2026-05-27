DROP INDEX IF EXISTS idx_trade_tracks_trend_15m;
DROP INDEX IF EXISTS idx_trade_tracks_trend_1h;
DROP INDEX IF EXISTS idx_trade_tracks_trend_4h;

ALTER TABLE trade_tracks DROP COLUMN IF EXISTS trend_15m;
ALTER TABLE trade_tracks DROP COLUMN IF EXISTS trend_1h;
ALTER TABLE trade_tracks DROP COLUMN IF EXISTS trend_4h;
