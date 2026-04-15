-- 为 trade_tracks 新增 opportunity_id 字段，关联 AI 分析结果与交易
ALTER TABLE trade_tracks ADD COLUMN IF NOT EXISTS opportunity_id INTEGER REFERENCES trading_opportunities(id);
CREATE INDEX IF NOT EXISTS idx_trade_tracks_opportunity_id ON trade_tracks(opportunity_id) WHERE opportunity_id IS NOT NULL;
