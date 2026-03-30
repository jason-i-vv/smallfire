-- 修复数据库中 K线数据的 CloseTime 计算错误问题
-- 原问题是在解析 Bybit API 时错误地使用了 item[6] 作为周期字段，
-- 导致 CloseTime 只比 OpenTime 晚 1 分钟，而不是对应周期的时长

-- 修复 15分钟周期的 K线
UPDATE klines
SET close_time = open_time + INTERVAL '15 minutes'
WHERE period = '15m'
  AND close_time < open_time + INTERVAL '14 minutes';

-- 修复 1小时周期的 K线
UPDATE klines
SET close_time = open_time + INTERVAL '1 hour'
WHERE period = '1h'
  AND close_time < open_time + INTERVAL '59 minutes';

-- 修复 5分钟周期的 K线
UPDATE klines
SET close_time = open_time + INTERVAL '5 minutes'
WHERE period = '5m'
  AND close_time < open_time + INTERVAL '4 minutes';

-- 修复 30分钟周期的 K线
UPDATE klines
SET close_time = open_time + INTERVAL '30 minutes'
WHERE period = '30m'
  AND close_time < open_time + INTERVAL '29 minutes';

-- 修复 4小时周期的 K线
UPDATE klines
SET close_time = open_time + INTERVAL '4 hours'
WHERE period = '4h'
  AND close_time < open_time + INTERVAL '239 minutes';

-- 修复 1天周期的 K线
UPDATE klines
SET close_time = open_time + INTERVAL '1 day'
WHERE period = '1d'
  AND close_time < open_time + INTERVAL '23 hours';

-- 修复 1分钟周期的 K线（如果有的话）
UPDATE klines
SET close_time = open_time + INTERVAL '1 minute'
WHERE period = '1m'
  AND close_time < open_time + INTERVAL '59 seconds';

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
