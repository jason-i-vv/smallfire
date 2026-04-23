#!/bin/bash
# 交易数据清理脚本
# 用途: 当开平仓逻辑发生变更后，清理旧的交易数据，确保新逻辑产生的数据干净
# 用法: ./scripts/cleanup_trading_data.sh "清理原因"
# 示例: ./scripts/cleanup_trading_data.sh "重构开平仓逻辑"

set -e

REASON="${1:-未指定原因}"
DATE=$(date +%Y%m%d)
CONTAINER="${DB_CONTAINER:-starfire-postgres}"
DB_USER="${DB_USER:-postgres}"
DB_NAME="${DB_NAME:-starfire_quant}"

echo "============================================"
echo "  交易数据清理工具"
echo "============================================"
echo "  日期: $(date '+%Y-%m-%d %H:%M:%S')"
echo "  原因: ${REASON}"
echo "  数据库: ${CONTAINER}/${DB_NAME}"
echo "============================================"

# 检查容器是否运行
if ! docker ps --format '{{.Names}}' | grep -q "^${CONTAINER}$"; then
    echo "错误: 数据库容器 ${CONTAINER} 未运行"
    echo "请先执行: make db-start"
    exit 1
fi

# 显示清理前的数据量
echo ""
echo "--- 清理前数据量 ---"
docker exec -i "${CONTAINER}" psql -U "${DB_USER}" -d "${DB_NAME}" <<EOF
SELECT 'trade_tracks' as table_name, COUNT(*) as count FROM trade_tracks
UNION ALL
SELECT 'trading_opportunities', COUNT(*) FROM trading_opportunities
UNION ALL
SELECT 'signal_type_stats', COUNT(*) FROM signal_type_stats
UNION ALL
SELECT 'signals', COUNT(*) FROM signals;
EOF

# 确认操作
echo ""
echo "⚠️  即将清理以上所有交易相关数据，此操作不可逆！"
read -p "确认执行？(y/N): " confirm
if [[ "${confirm}" != "y" && "${confirm}" != "Y" ]]; then
    echo "已取消"
    exit 0
fi

# 执行清理
echo ""
echo "--- 开始清理 ---"
docker exec -i "${CONTAINER}" psql -U "${DB_USER}" -d "${DB_NAME}" <<EOF
BEGIN;

-- 1. 清理 trade_tracks (必须先于 trading_opportunities，因为有外键)
DELETE FROM trade_tracks;

-- 2. 清理 trading_opportunities
DELETE FROM trading_opportunities;

-- 3. 清理 signal_type_stats (反馈闭环统计数据)
DELETE FROM signal_type_stats;

-- 4. 清理 signals (信号数据)
DELETE FROM signals;

-- 5. 清理 notifications (通知记录，依赖 signals)
DELETE FROM notifications;

COMMIT;
EOF

# 验证清理结果
echo ""
echo "--- 清理后数据量 ---"
docker exec -i "${CONTAINER}" psql -U "${DB_USER}" -d "${DB_NAME}" <<EOF
SELECT 'trade_tracks' as table_name, COUNT(*) as remaining FROM trade_tracks
UNION ALL
SELECT 'trading_opportunities', COUNT(*) FROM trading_opportunities
UNION ALL
SELECT 'signal_type_stats', COUNT(*) FROM signal_type_stats
UNION ALL
SELECT 'signals', COUNT(*) FROM signals;
EOF

# 记录归档脚本
ARCHIVE_FILE="db-scripts/cleanup_trading_data_${DATE}.sql"
cat > "${ARCHIVE_FILE}" <<ARCHIVE
-- 清理交易数据
-- 执行日期: $(date '+%Y-%m-%d')
-- 原因: ${REASON}

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
ARCHIVE

echo ""
echo "============================================"
echo "  清理完成！"
echo "  归档已保存到: ${ARCHIVE_FILE}"
echo "============================================"
