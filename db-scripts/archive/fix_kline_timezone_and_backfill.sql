-- ============================================================
-- 修复 K 线数据时区问题并回补缺失数据
-- 日期: 2026-04-03
-- ============================================================
-- 问题:
--   1. open_time/close_time 存储的是 UTC+8 本地时间，
--      但 timestamp without time zone 列无时区信息，API 按 UTC 返回导致时间偏移 8 小时
--   2. 同步服务停机期间产生的 K 线缺口未回补
-- 修复:
--   1. 将所有 klines 的 open_time/close_time 减去 8 小时，转为真实 UTC
--   2. 代码层面已修复，新写入的数据统一使用 UTC
-- ============================================================

BEGIN;

-- 步骤 1: 修正所有 K 线的时区（open_time - 8h, close_time - 8h）
UPDATE klines
SET open_time = open_time - INTERVAL '8 hours',
    close_time = close_time - INTERVAL '8 hours'
WHERE open_time IS NOT NULL;

-- 验证修正结果
SELECT 'timezone_fix' AS step,
       COUNT(*) AS total_rows,
       MIN(open_time) AS earliest_open_time,
       MAX(open_time) AS latest_open_time
FROM klines
WHERE period = '1h';

-- 步骤 2: 检查修正后的连续性（以 symbol_id=193, period=1h 为例）
WITH ordered_klines AS (
    SELECT open_time,
           LAG(open_time) OVER (ORDER BY open_time) AS prev_open_time
    FROM klines
    WHERE symbol_id = 193 AND period = '1h'
)
SELECT 'continuity_check' AS step,
       COUNT(*) AS gap_count
FROM ordered_klines
WHERE prev_open_time IS NOT NULL
  AND open_time - prev_open_time > INTERVAL '1 hour 30 minutes';

COMMIT;

-- ============================================================
-- 注意事项:
-- 1. 执行前请备份数据: pg_dump -U postgres starfire_quant > backup_before_tz_fix.sql
-- 2. 修正后需要重启同步服务，新写入的数据将使用正确的 UTC 时间
-- 3. 缺失的 K 线数据将在同步服务重启后自动通过 backfill 逻辑回补
--    （最多回补 168 小时 = 7 天的数据）
-- ============================================================
