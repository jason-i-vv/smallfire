DROP INDEX IF EXISTS idx_trade_tracks_opportunity_id;
ALTER TABLE trade_tracks DROP COLUMN IF EXISTS opportunity_id;
