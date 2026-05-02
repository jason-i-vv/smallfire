-- 005: 新增交易来源字段，支持区分 paper trading 和 bybit testnet
-- 执行时间: 2026-04-24

-- 交易来源: paper=纸上交易, testnet=Bybit测试网
ALTER TABLE trade_tracks ADD COLUMN IF NOT EXISTS trade_source VARCHAR(20) DEFAULT 'paper';
-- 交易所订单ID（testnet 交易时存储 Bybit 订单 ID）
ALTER TABLE trade_tracks ADD COLUMN IF NOT EXISTS exchange_order_id VARCHAR(50);

CREATE INDEX IF NOT EXISTS idx_tracks_trade_source ON trade_tracks(trade_source);
CREATE INDEX IF NOT EXISTS idx_tracks_exchange_order_id ON trade_tracks(exchange_order_id);

-- 将现有数据标记为 paper
UPDATE trade_tracks SET trade_source = 'paper' WHERE trade_source IS NULL;
