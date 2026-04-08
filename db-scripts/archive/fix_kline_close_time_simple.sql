-- 先禁用触发器
ALTER TABLE klines DISABLE TRIGGER trigger_update_klines;

-- 修复 15分钟周期的 K线
UPDATE klines
SET close_time = open_time + INTERVAL '15 minutes'
WHERE period = '15m';

-- 修复 1小时周期的 K线
UPDATE klines
SET close_time = open_time + INTERVAL '1 hour'
WHERE period = '1h';

-- 重新启用触发器
ALTER TABLE klines ENABLE TRIGGER trigger_update_klines;

-- 查询修复后的统计信息
SELECT
    period,
    COUNT(*) AS total_klines,
    COUNT(CASE WHEN close_time >= open_time +
        CASE
            WHEN period = '1m' THEN INTERVAL '1 minute'
            WHEN period = '5m' THEN INTERVAL '5 minutes'
            WHEN period = '15m' THEN INTERVAL '15 minutes'
            WHEN period = '30m' THEN INTERVAL '30 minutes'
            WHEN period = '1h' THEN INTERVAL '1 hour'
            WHEN period = '4h' THEN INTERVAL '4 hours'
            WHEN period = '1d' THEN INTERVAL '1 day'
            ELSE INTERVAL '1 minute'
        END
    THEN 1 ELSE NULL END) AS valid_klines,
    COUNT(CASE WHEN close_time < open_time +
        CASE
            WHEN period = '1m' THEN INTERVAL '1 minute'
            WHEN period = '5m' THEN INTERVAL '5 minutes'
            WHEN period = '15m' THEN INTERVAL '15 minutes'
            WHEN period = '30m' THEN INTERVAL '30 minutes'
            WHEN period = '1h' THEN INTERVAL '1 hour'
            WHEN period = '4h' THEN INTERVAL '4 hours'
            WHEN period = '1d' THEN INTERVAL '1 day'
            ELSE INTERVAL '1 minute'
        END
    THEN 1 ELSE NULL END) AS invalid_klines
FROM klines
GROUP BY period
ORDER BY period;
