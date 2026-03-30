-- 补充索引和触发器（表已在001_init.sql中创建）

-- 补充索引
CREATE INDEX IF NOT EXISTS idx_symbols_is_tracking ON symbols(is_tracking);

-- 更新时间戳函数
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    -- 检查 NEW 是否包含 updated_at 字段
    -- 使用异常处理来避免错误
    BEGIN
        NEW.updated_at = CURRENT_TIMESTAMP;
    EXCEPTION
        WHEN others THEN
            -- 字段不存在时，直接返回 NEW 不做修改
            RETURN NEW;
    END;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- 创建更新触发器（使用DO语句检查是否存在）
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trigger_update_markets') THEN
        CREATE TRIGGER trigger_update_markets
            BEFORE UPDATE ON markets
            FOR EACH ROW
            EXECUTE FUNCTION update_updated_at();
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trigger_update_symbols') THEN
        CREATE TRIGGER trigger_update_symbols
            BEFORE UPDATE ON symbols
            FOR EACH ROW
            EXECUTE FUNCTION update_updated_at();
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trigger_update_klines') THEN
        CREATE TRIGGER trigger_update_klines
            BEFORE UPDATE ON klines
            FOR EACH ROW
            EXECUTE FUNCTION update_updated_at();
    END IF;
END $$;
