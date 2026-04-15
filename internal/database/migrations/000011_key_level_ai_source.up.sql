-- 000011: 扩展 key_levels 表，支持 AI 识别的关键价位
-- 新增 source 列区分算法识别和 AI 识别
-- 新增 strength 列存储 AI 评估的价位强度
-- 新增 ai_reason 列存储 AI 的识别理由

ALTER TABLE key_levels ADD COLUMN IF NOT EXISTS source VARCHAR(20) DEFAULT 'algorithm';
ALTER TABLE key_levels ADD COLUMN IF NOT EXISTS strength INTEGER DEFAULT 1;
ALTER TABLE key_levels ADD COLUMN IF NOT EXISTS ai_reason TEXT;

CREATE INDEX IF NOT EXISTS idx_key_levels_source ON key_levels(source);
