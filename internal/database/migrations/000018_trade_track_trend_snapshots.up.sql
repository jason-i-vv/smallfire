-- Store trend snapshots on each trade at entry time.
ALTER TABLE trade_tracks ADD COLUMN IF NOT EXISTS trend_4h VARCHAR(20);
ALTER TABLE trade_tracks ADD COLUMN IF NOT EXISTS trend_1h VARCHAR(20);
ALTER TABLE trade_tracks ADD COLUMN IF NOT EXISTS trend_15m VARCHAR(20);

CREATE INDEX IF NOT EXISTS idx_trade_tracks_trend_4h ON trade_tracks(trend_4h) WHERE trend_4h IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_trade_tracks_trend_1h ON trade_tracks(trend_1h) WHERE trend_1h IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_trade_tracks_trend_15m ON trade_tracks(trend_15m) WHERE trend_15m IS NOT NULL;
