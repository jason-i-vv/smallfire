-- 补充索引

-- 全索引（001_init_schema 中 idx_symbols_tracking 是部分索引）
CREATE INDEX IF NOT EXISTS idx_symbols_is_tracking ON symbols(is_tracking);
