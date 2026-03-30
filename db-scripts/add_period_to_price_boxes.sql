-- 为 price_boxes 表添加 period 字段
-- 这个字段用于区分不同周期的箱体数据

-- 先禁用触发器，避免更新时的问题
ALTER TABLE price_boxes DISABLE TRIGGER trigger_update_price_boxes_updated_at;

-- 添加 period 字段
ALTER TABLE price_boxes ADD COLUMN IF NOT EXISTS period VARCHAR(10);

-- 更新现有数据的 period 字段（如果有数据的话）
-- 注意：由于之前没有记录 period，我们需要根据数据来推断，或者设为默认值
-- 这里我们设置为默认值 '15m'，如果有其他周期的数据需要手动调整
UPDATE price_boxes SET period = '15m' WHERE period IS NULL;

-- 将 period 字段设为非空
ALTER TABLE price_boxes ALTER COLUMN period SET NOT NULL;

-- 添加索引以提高查询性能
CREATE INDEX IF NOT EXISTS idx_boxes_symbol_period ON price_boxes(symbol_id, period);

-- 重新启用触发器
ALTER TABLE price_boxes ENABLE TRIGGER trigger_update_price_boxes_updated_at;

-- 验证修改成功
SELECT
    column_name,
    data_type,
    is_nullable,
    column_default
FROM information_schema.columns
WHERE table_name = 'price_boxes' AND column_name = 'period';

-- 查看索引
SELECT indexname, indexdef
FROM pg_indexes
WHERE tablename = 'price_boxes'
ORDER BY indexname;
