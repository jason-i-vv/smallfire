-- 回滚：信号类型统计表
DROP TABLE IF EXISTS signal_type_stats;

-- 回滚：交易机会表
DROP TABLE IF EXISTS trading_opportunities;

-- 回滚：信号扩展字段
ALTER TABLE signals DROP COLUMN IF EXISTS confluence_info;
ALTER TABLE signals DROP COLUMN IF EXISTS valid_until;
ALTER TABLE signals DROP COLUMN IF EXISTS score_details;
ALTER TABLE signals DROP COLUMN IF EXISTS score;
