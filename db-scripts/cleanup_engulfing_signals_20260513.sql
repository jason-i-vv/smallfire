-- 清理阳包阴/阴包阳吞没信号
-- 执行日期: 2026-05-13
-- 原因: 禁用低价值吞没形态策略，不再展示或使用 engulfing_bullish / engulfing_bearish

BEGIN;

CREATE TEMP TABLE _engulfing_signal_ids AS
SELECT id
FROM signals
WHERE signal_type IN ('engulfing_bullish', 'engulfing_bearish');

CREATE TEMP TABLE _engulfing_opportunity_ids AS
SELECT id
FROM trading_opportunities
WHERE confluence_directions IS NOT NULL
  AND EXISTS (
    SELECT 1
    FROM unnest(confluence_directions) AS d
    WHERE d = 'engulfing_bullish'
       OR d = 'engulfing_bearish'
       OR d LIKE 'engulfing_bullish:%'
       OR d LIKE 'engulfing_bearish:%'
  );

DELETE FROM notifications
WHERE signal_id IN (SELECT id FROM _engulfing_signal_ids);

UPDATE trade_tracks
SET signal_id = NULL
WHERE signal_id IN (SELECT id FROM _engulfing_signal_ids);

UPDATE trade_tracks
SET opportunity_id = NULL
WHERE opportunity_id IN (SELECT id FROM _engulfing_opportunity_ids);

DELETE FROM trading_opportunities
WHERE id IN (SELECT id FROM _engulfing_opportunity_ids);

DELETE FROM signal_type_stats
WHERE signal_type IN ('engulfing_bullish', 'engulfing_bearish');

DELETE FROM signals
WHERE id IN (SELECT id FROM _engulfing_signal_ids);

COMMIT;

SELECT 'signals' AS table_name, COUNT(*) AS remaining
FROM signals
WHERE signal_type IN ('engulfing_bullish', 'engulfing_bearish')
UNION ALL
SELECT 'signal_type_stats', COUNT(*)
FROM signal_type_stats
WHERE signal_type IN ('engulfing_bullish', 'engulfing_bearish')
UNION ALL
SELECT 'trading_opportunities', COUNT(*)
FROM trading_opportunities
WHERE confluence_directions IS NOT NULL
  AND EXISTS (
    SELECT 1
    FROM unnest(confluence_directions) AS d
    WHERE d = 'engulfing_bullish'
       OR d = 'engulfing_bearish'
       OR d LIKE 'engulfing_bullish:%'
       OR d LIKE 'engulfing_bearish:%'
  );
