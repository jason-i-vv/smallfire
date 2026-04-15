-- 模拟交易不关联信号，signal_id 改为可 NULL
ALTER TABLE trade_tracks ALTER COLUMN signal_id DROP NOT NULL;
