-- 清理交易数据：交易记录、交易机会、信号统计
-- 执行日期: 2026-04-16
-- 原因: 旧数据中 opportunity_id 丢失导致反馈闭环失效，且止盈止损设置异常

BEGIN;

-- 1. 清理 trade_tracks (必须先于 trading_opportunities，因为有外键)
DELETE FROM trade_tracks;

-- 2. 清理 trading_opportunities
DELETE FROM trading_opportunities;

-- 3. 清理 signal_type_stats (反馈闭环统计数据)
DELETE FROM signal_type_stats;

COMMIT;

-- 验证清理结果
SELECT 'trade_tracks' as table_name, COUNT(*) as remaining FROM trade_tracks
UNION ALL
SELECT 'trading_opportunities', COUNT(*) FROM trading_opportunities
UNION ALL
SELECT 'signal_type_stats', COUNT(*) FROM signal_type_stats
UNION ALL
SELECT 'signals', COUNT(*) FROM signals;
