-- 为 price_boxes 表添加 period 字段（如果不存在）
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'price_boxes' AND column_name = 'period'
    ) THEN
        ALTER TABLE price_boxes ADD COLUMN period VARCHAR(10);
        UPDATE price_boxes SET period = '15m' WHERE period IS NULL;
        ALTER TABLE price_boxes ALTER COLUMN period SET NOT NULL;
        CREATE INDEX IF NOT EXISTS idx_boxes_symbol_period ON price_boxes(symbol_id, period);
        RAISE NOTICE '已添加 period 列到 price_boxes 表';
    ELSE
        RAISE NOTICE 'price_boxes 表的 period 列已存在，跳过';
    END IF;
END
$$;
