-- 2025-05-08: 交易机会增加 regime 和 strategy_type 字段
-- 原因：统计查询的 regime 和 strategy_type 改为从机会表取值，
--        而不是实时计算 symbols.trend_4h 或关联 signals.source_type

-- 新增列
ALTER TABLE trading_opportunities ADD COLUMN IF NOT EXISTS regime VARCHAR(20) DEFAULT '震荡';
ALTER TABLE trading_opportunities ADD COLUMN IF NOT EXISTS strategy_type VARCHAR(50) DEFAULT 'unknown';

-- 回填 regime：根据 symbol 趋势 + 交易方向计算
UPDATE trading_opportunities o
SET regime = CASE
    WHEN (s.trend_4h = 'bullish' AND o.direction = 'long')
        OR (s.trend_4h = 'bearish' AND o.direction = 'short') THEN '顺势'
    WHEN (s.trend_4h = 'bullish' AND o.direction = 'short')
        OR (s.trend_4h = 'bearish' AND o.direction = 'long') THEN '逆势'
    ELSE '震荡'
END
FROM symbols s WHERE o.symbol_id = s.id;

-- 回填 strategy_type：从 confluence_directions 推导信号来源类型
UPDATE trading_opportunities o
SET strategy_type = CASE
    WHEN EXISTS (SELECT 1 FROM unnest(o.confluence_directions) d WHERE d LIKE 'box_%') THEN 'box'
    WHEN EXISTS (SELECT 1 FROM unnest(o.confluence_directions) d WHERE d LIKE 'trend_%') THEN 'trend'
    WHEN EXISTS (SELECT 1 FROM unnest(o.confluence_directions) d WHERE d LIKE 'macd%') THEN 'macd'
    WHEN EXISTS (SELECT 1 FROM unnest(o.confluence_directions) d WHERE d LIKE 'engulfing_%' OR d LIKE 'momentum_%' OR d LIKE 'morning_%' OR d LIKE 'evening_%') THEN 'candlestick'
    WHEN EXISTS (SELECT 1 FROM unnest(o.confluence_directions) d WHERE d LIKE '%wick_%' OR d LIKE 'fake_%') THEN 'wick'
    WHEN EXISTS (SELECT 1 FROM unnest(o.confluence_directions) d WHERE d LIKE 'volume_%' OR d LIKE 'price_surge%' OR d LIKE 'resistance_%' OR d LIKE 'support_%') THEN 'volume'
    ELSE 'unknown'
END
WHERE o.confluence_directions IS NOT NULL AND array_length(o.confluence_directions, 1) > 0;
