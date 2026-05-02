#!/bin/bash
# 修复 testnet unknown exit_reason 记录
# 1. 先把所有 unknown 改为 stop_loss/take_profit
# 2. 逐个查 Bybit API 用 order_id 匹配，更新 entry_price/exit_price/pnl/fees

API_KEY="DatS4UH1Leyg9qG2Kc"
API_SECRET="UlUKA1qDeVRa3XYAuzDbGVyWqZZICWvvmY6y"
BASE_URL="https://api-demo.bybit.com"
RECV_WINDOW="5000"

sign_and_call() {
  local path="$1"
  local timestamp=$(date +%s)000
  local payload="${timestamp}${API_KEY}${RECV_WINDOW}${path#*\?}"
  local signature=$(printf "%s" "$payload" | openssl dgst -sha256 -hmac "$API_SECRET" | awk '{print $2}')
  curl -s -H "X-BAPI-API-KEY: $API_KEY" \
       -H "X-BAPI-TIMESTAMP: $timestamp" \
       -H "X-BAPI-SIGN: $signature" \
       -H "X-BAPI-RECV-WINDOW: $RECV_WINDOW" \
       "${BASE_URL}${path}"
}

# 获取所有有 exchange_order_id 的 testnet closed 记录
echo "=== 查询所有需要修复的记录 ==="
RECORDS=$(sudo docker exec starfire-postgres psql -U postgres -d starfire_quant -t -c "
SELECT t.id, s.symbol_code, t.exchange_order_id, t.entry_price, t.exit_price, t.pnl
FROM trade_tracks t
JOIN symbols s ON t.symbol_id = s.id
WHERE t.trade_source = 'testnet' AND t.status = 'closed'
AND t.exchange_order_id IS NOT NULL AND t.exchange_order_id != ''
ORDER BY t.id;" 2>/dev/null)

TOTAL=0
UPDATED=0
NOT_FOUND=0

echo "$RECORDS" | while IFS='|' read -r id symbol_code order_id local_entry local_exit local_pnl; do
  id=$(echo "$id" | xargs)
  symbol_code=$(echo "$symbol_code" | xargs)
  order_id=$(echo "$order_id" | xargs)
  local_entry=$(echo "$local_entry" | xargs)
  local_exit=$(echo "$local_exit" | xargs)
  local_pnl=$(echo "$local_pnl" | xargs)
  TOTAL=$((TOTAL+1))

  # 用 order_id 查 execution 记录获取真实成交价
  path="/v5/execution/list?category=linear&symbol=${symbol_code}&orderId=${order_id}"
  resp=$(sign_and_call "$path")

  # 解析 avgPrice, qty, fee
  # 格式: {"retCode":0,"result":{"list":[{"execId":...,"execPrice":...,"execQty":...,"execFee":...}]}}
  echo "$resp" | grep -q '"retCode":0' || {
    echo "  [$id/$symbol_code] order not found in executions, try closed-pnl"
    # 尝试 closed-pnl
    path2="/v5/position/closed-pnl?category=linear&symbol=${symbol_code}&limit=20"
    resp2=$(sign_and_call "$path2")
    echo "$resp2" | grep -q '"retCode":0' || {
      echo "  [$id/$symbol_code] closed-pnl not found"
      NOT_FOUND=$((NOT_FOUND+1))
      continue
    }
    # 从 closed-pnl 中找匹配的记录
    # 简化处理：取第一条（最新）
    echo "$resp2" | python3 -c "
import sys,json
d=json.load(sys.stdin)
for t in d.get('result',{}).get('list',[]):
  if str(t.get('orderId','')) == '$order_id':
    print('PNLFOUND', t['closedPnl'], t['entryPrice'], t['exitPrice'], t['fee'])
    break
" 2>/dev/null | read -r flag cpnl entry exit fee || {
      echo "  [$id/$symbol_code] order_id not matched in closed-pnl"
      NOT_FOUND=$((NOT_FOUND+1))
      continue
    }
    continue
  }

  # 从 execution list 解析
  echo "$resp" | python3 -c "
import sys,json
d=json.load(sys.stdin)
total_val=0; total_qty=0; total_fee=0
for e in d.get('result',{}).get('list',[]):
  price=float(e.get('execPrice',0))
  qty=float(e.get('execQty',0))
  fee=float(e.get('execFee',0))
  total_val+=price*qty
  total_qty+=qty
  total_fee+=abs(fee)
if total_qty>0:
  avg=total_val/total_qty
  print('EXECFOUND', avg, total_qty, total_fee)
" 2>/dev/null | read -r flag avg_price fill_qty fill_fee || {
    echo "  [$id/$symbol_code] failed to parse execution"
    continue
  }

  # 计算 PnL（需要知道 direction）
  direction=$(sudo docker exec starfire-postgres psql -U postgres -d starfire_quant -t -c "SELECT direction FROM trade_tracks WHERE id=$id;" 2>/dev/null | xargs)
  position_value=$(sudo docker exec starfire-postgres psql -U postgres -d starfire_quant -t -c "SELECT position_value FROM trade_tracks WHERE id=$id;" 2>/dev/null | xargs)

  if [ "$direction" = "long" ]; then
    pnl=$(echo "$avg_price $local_entry $fill_qty" | awk '{printf "%.8f", ($1 - $2) * $3}')
  else
    pnl=$(echo "$avg_price $local_entry $fill_qty" | awk '{printf "%.8f", ($2 - $1) * $3}')
  fi

  total_fee=$(echo "$fill_fee" | awk '{printf "%.8f", $1}')
  pnl=$(echo "$pnl $total_fee" | awk '{printf "%.8f", $1 - $2}')

  echo "  [$id/$symbol_code] exec: avg=$avg_price qty=$fill_qty fee=$fill_fee pnl=$pnl"
  # 更新数据库
  sudo docker exec starfire-postgres psql -U postgres -d starfire_quant -c "
  UPDATE trade_tracks SET
    entry_price=$avg_price,
    exit_price=$fill_qty,
    pnl=$pnl,
    fees=$fill_fee,
    quantity=$fill_qty,
    updated_at=NOW()
  WHERE id=$id;" 2>/dev/null

  UPDATED=$((UPDATED+1))
  echo "  --> Updated $id"
done

echo ""
echo "=== 完成: total=$TOTAL updated=$UPDATED not_found=$NOT_FOUND ==="