-- 修复 testnet 持仓的 position_value 和 pnl_percent
-- 问题：position_value 存储了杠杆后的仓位价值（leverage * tradeAmount），
--       导致 PnL% = pnl / positionValue 只有实际 ROE 的 1/leverage
-- 修复：将 position_value 改为实际保证金（tradeAmount = 100 USDT）
--       这样 PnL% = pnl / margin = 实际 ROE，与 Bybit 显示一致

-- 1. 修复所有 testnet 持仓的 position_value（除以杠杆倍数 2）
UPDATE trade_tracks
SET position_value = position_value / 2,
    updated_at = NOW()
WHERE trade_source = 'testnet'
  AND position_value = 200;  -- leverage(2) * tradeAmount(100)

-- 2. 重新计算已平仓记录的 pnl_percent
UPDATE trade_tracks
SET pnl_percent = CASE
        WHEN position_value = 0 THEN 0
        ELSE pnl / position_value
    END,
    updated_at = NOW()
WHERE trade_source = 'testnet'
  AND status = 'closed'
  AND pnl IS NOT NULL
  AND position_value > 0;

-- 3. 重新计算持仓中的 unrealized_pnl_pct（会在下一次监控轮询时自动更新，此处兜底）
UPDATE trade_tracks
SET unrealized_pnl_pct = CASE
        WHEN position_value = 0 THEN 0
        ELSE unrealized_pnl / position_value
    END,
    updated_at = NOW()
WHERE trade_source = 'testnet'
  AND status = 'open'
  AND unrealized_pnl IS NOT NULL
  AND position_value > 0;
