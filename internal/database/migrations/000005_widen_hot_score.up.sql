-- 扩大 hot_score 字段精度，避免 A 股成交额溢出
-- DECIMAL(10,2) 最大值 99999999.99，A 股大盘股日成交额可达数十亿
ALTER TABLE symbols ALTER COLUMN hot_score TYPE DECIMAL(18, 2);
