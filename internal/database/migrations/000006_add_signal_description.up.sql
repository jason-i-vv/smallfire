-- 为 signals 表添加 description 字段
ALTER TABLE signals ADD COLUMN IF NOT EXISTS description VARCHAR(255) DEFAULT '';
