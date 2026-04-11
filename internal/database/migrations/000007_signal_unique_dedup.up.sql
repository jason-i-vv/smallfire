-- 信号去重唯一索引：同一标的+信号类型+周期+K线时间只允许一条信号
CREATE UNIQUE INDEX IF NOT EXISTS idx_signals_unique_dedup
    ON signals (symbol_id, signal_type, period, kline_time);
