-- 清理数据库中的重复箱体数据
-- 这个脚本会找到并删除重复的活跃箱体，只保留时间最早的那个

-- 先统计一下当前有多少活跃箱体
SELECT '当前活跃箱体总数' as info, COUNT(*) as count FROM price_boxes WHERE status = 'active';

-- 创建临时表来存储需要删除的箱体ID
CREATE TEMP TABLE temp_boxes_to_delete (
    id int NOT NULL
);

-- 找出价格重叠度>80%的重复箱体
WITH box_comparisons AS (
    SELECT
        b1.id as box1_id,
        b2.id as box2_id,
        b1.symbol_id,
        b1.period,
        b1.start_time as b1_start,
        b2.start_time as b2_start,
        b1.high_price as b1_high,
        b1.low_price as b1_low,
        b2.high_price as b2_high,
        b2.low_price as b2_low,
        -- 计算价格重叠度
        CASE
            WHEN b1.high_price < b2.low_price OR b2.high_price < b1.low_price THEN 0
            ELSE (LEAST(b1.high_price, b2.high_price) - GREATEST(b1.low_price, b2.low_price)) /
                 (GREATEST(b1.high_price, b2.high_price) - LEAST(b1.low_price, b2.low_price))
        END as price_overlap
    FROM price_boxes b1
    JOIN price_boxes b2 ON
        b1.symbol_id = b2.symbol_id AND
        b1.period = b2.period AND
        b1.id < b2.id AND
        b1.status = 'active' AND
        b2.status = 'active'
),
-- 找出价格重叠度>80%的箱体对，保留时间较早的那个，删除时间较晚的那个
duplicate_pairs AS (
    SELECT
        CASE
            WHEN b1_start < b2_start THEN box2_id
            ELSE box1_id
        END as id_to_delete,
        CASE
            WHEN b1_start < b2_start THEN box1_id
            ELSE box2_id
        END as id_to_keep
    FROM box_comparisons
    WHERE price_overlap > 0.8
)
-- 插入到临时表中，确保只删除一次
INSERT INTO temp_boxes_to_delete (id)
SELECT DISTINCT id_to_delete
FROM duplicate_pairs
-- 确保不删除我们要保留的箱体
WHERE id_to_delete NOT IN (
    SELECT DISTINCT id_to_keep FROM duplicate_pairs
);

-- 统计要删除的数量
SELECT '将要删除的重复箱体数量' as info, COUNT(*) as count FROM temp_boxes_to_delete;

-- 执行删除
DELETE FROM price_boxes WHERE id IN (SELECT id FROM temp_boxes_to_delete);

-- 显示最终统计
SELECT '剩余活跃箱体数量' as info, COUNT(*) as count FROM price_boxes WHERE status = 'active';

-- 删除临时表
DROP TABLE temp_boxes_to_delete;
