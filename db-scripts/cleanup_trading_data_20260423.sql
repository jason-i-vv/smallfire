-- 清理交易数据
-- 执行日期: 2026-04-23
-- 原因: 重构开平仓逻辑

BEGIN;

DELETE FROM trade_tracks;
DELETE FROM trading_opportunities;
DELETE FROM signal_type_stats;
DELETE FROM signals;
DELETE FROM notifications;

COMMIT;

SELECT 'trade_tracks' as table_name, COUNT(*) as remaining FROM trade_tracks
UNION ALL
SELECT 'trading_opportunities', COUNT(*) FROM trading_opportunities
UNION ALL
SELECT 'signal_type_stats', COUNT(*) FROM signal_type_stats
UNION ALL
SELECT 'signals', COUNT(*) FROM signals;
