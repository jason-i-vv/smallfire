-- 添加监测表的 symbol_code 字段
ALTER TABLE monitorings
ADD COLUMN symbol_code VARCHAR(30) NOT NULL DEFAULT '';

-- 为 symbol_code 添加索引
CREATE INDEX IF NOT EXISTS idx_monitorings_symbol_code ON monitorings(symbol_code);

-- 更新现有数据的 symbol_code（通过关联 symbols 表）
UPDATE monitorings m
SET symbol_code = s.symbol_code
FROM symbols s
WHERE m.symbol_id = s.id;

-- 移除默认值约束（可选，看需要）
ALTER TABLE monitorings ALTER COLUMN symbol_code DROP DEFAULT;
