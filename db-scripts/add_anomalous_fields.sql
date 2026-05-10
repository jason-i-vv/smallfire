-- 添加异常状态相关字段
-- anomalous_reason: 记录持仓被标记为异常的原因
ALTER TABLE trade_tracks ADD COLUMN IF NOT EXISTS anomalous_reason TEXT;

-- 更新 status 的 comment 以包含 anomalous
COMMENT ON COLUMN trade_tracks.status IS 'open, closed, anomalous';
COMMENT ON COLUMN trade_tracks.anomalous_reason IS '异常原因描述，仅 status=anomalous 时有值';
