-- 恢复 signal_id 的 NOT NULL 约束
ALTER TABLE trade_tracks ALTER COLUMN signal_id SET NOT NULL;
