-- 移除 AI 相关字段
ALTER TABLE key_levels DROP COLUMN IF EXISTS source;
ALTER TABLE key_levels DROP COLUMN IF EXISTS strength;
ALTER TABLE key_levels DROP COLUMN IF EXISTS ai_reason;
DROP INDEX IF EXISTS idx_key_levels_source;
