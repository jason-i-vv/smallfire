-- 清理交易数据：交易记录、交易机会、信号统计
-- 执行日期: 2026-04-17
-- 原因: 各策略盈亏比优化后，需要重新积累数据验证效果
--       旧数据中的 SL/TP 值为修复前计算，需要新数据验证 ATR 止盈止损

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
