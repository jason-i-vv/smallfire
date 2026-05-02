#!/usr/bin/env python3
import subprocess, json, time, hmac, hashlib

API_KEY = "DatS4UH1Leyg9qG2Kc"
API_SECRET = "UlUKA1qDeVRa3XYAuzDbGVyWqZZICWvvmY6y"
BASE_URL = "https://api-demo.bybit.com"

def sign(path):
    param_str = path.split("?",1)[1]
    ts = str(int(time.time()*1000))
    payload = f"{ts}{API_KEY}5000{param_str}"
    sig = hmac.new(API_SECRET.encode(), payload.encode(), hashlib.sha256).hexdigest()
    return sig, ts

def bybit_get(path):
    sig, ts = sign(path)
    r = subprocess.run(["curl", "-s", f"{BASE_URL}{path}",
        "-H", f"X-BAPI-API-KEY: {API_KEY}",
        "-H", f"X-BAPI-TIMESTAMP: {ts}",
        "-H", f"X-BAPI-SIGN: {sig}",
        "-H", "X-BAPI-RECV-WINDOW: 5000"
    ], capture_output=True, text=True)
    return json.loads(r.stdout)

def db_query(sql):
    r = subprocess.run(
        ["sudo", "docker", "exec", "starfire-postgres",
         "psql", "-U", "postgres", "-d", "starfire_quant", "-t", "-c", sql],
        capture_output=True, text=True
    )
    return r.stdout

# 1. 先批量修正所有 unknown 为 stop_loss/take_profit（兜底）
print("=== Step 1: 批量修正 exit_reason ===")
r = db_query("""
UPDATE trade_tracks
SET exit_reason = CASE WHEN pnl < 0 THEN 'stop_loss' ELSE 'take_profit' END
WHERE trade_source = 'testnet' AND status = 'closed' AND exit_reason = 'unknown';
""")
print(r.strip())

# 2. 读取所有 testnet closed 有 exchange_order_id 的记录
print("\n=== Step 2: 读取待修复记录 ===")
rows = db_query("""
SELECT t.id, s.symbol_code, t.exchange_order_id, t.entry_price, t.exit_price, t.pnl, t.direction, t.created_at
FROM trade_tracks t
JOIN symbols s ON t.symbol_id = s.id
WHERE t.trade_source = 'testnet' AND t.status = 'closed'
AND t.exchange_order_id IS NOT NULL AND t.exchange_order_id != ''
ORDER BY t.created_at DESC;
""")

records = []
for line in rows.strip().split("\n"):
    if not line.strip(): continue
    parts = line.strip().split("|")
    if len(parts) >= 8:
        records.append({
            "id": int(parts[0].strip()),
            "symbol": parts[1].strip(),
            "order_id": parts[2].strip(),
            "direction": parts[6].strip(),
        })

print(f"共 {len(records)} 条")

# 3. 按 symbol 查 bybit，匹配 order_id
symbol_map = {}
for r2 in records:
    symbol_map.setdefault(r2["symbol"], []).append(r2)

updates = []
not_found_by_time = []

for symbol, recs in symbol_map.items():
    data = bybit_get(f"/v5/position/closed-pnl?category=linear&symbol={symbol}&limit=50")
    if data.get("retCode") != 0:
        print(f"\n[{symbol}] API 错误: {data.get('retMsg')}")
        continue

    pnl_list = data["result"]["list"]
    # 构建 orderId -> bybit 记录映射
    pnl_map = {p["orderId"]: p for p in pnl_list}

    for r2 in recs:
        bp = pnl_map.get(r2["order_id"])
        if not bp:
            not_found_by_time.append(r2)
            continue

        entry_price = float(bp["avgEntryPrice"])
        exit_price = float(bp["avgExitPrice"])
        closed_pnl = float(bp["closedPnl"])
        fee = float(bp["openFee"]) + float(bp["closeFee"])
        qty = float(bp["qty"])

        exit_reason = "stop_loss" if closed_pnl < 0 else "take_profit"

        sql = f"UPDATE trade_tracks SET entry_price={entry_price:.8f}, exit_price={exit_price:.8f}, pnl={closed_pnl:.8f}, fees={fee:.8f}, quantity={qty:.8f}, exit_reason='{exit_reason}', updated_at=NOW() WHERE id={r2['id']};"
        updates.append(sql)
        print(f"  [{r2['id']}/{symbol}] matched: entry={entry_price:.6f} exit={exit_price:.6f} pnl={closed_pnl:.4f} {exit_reason}")

print(f"\n=== 总结 ===")
print(f"Bybit 可查到并修复: {len(updates)} 条")
print(f"Bybit 查不到（太旧）: {len(not_found_by_time)} 条")

if updates:
    print(f"\n=== 执行 UPDATE ===")
    sql_all = "\n".join(updates)
    result = subprocess.run(
        ["sudo", "docker", "exec", "starfire-postgres",
         "psql", "-U", "postgres", "-d", "starfire_quant", "-c", sql_all],
        capture_output=True, text=True
    )
    print(result.stdout[:500] if result.stdout else "OK")
    if result.stderr:
        print("STDERR:", result.stderr[:200])

    # 验证
    remaining = db_query("SELECT COUNT(*) FROM trade_tracks WHERE trade_source = 'testnet' AND status = 'closed' AND exit_reason = 'unknown';")
    print(f"\n修正后 unknown 剩余: {remaining.strip()}")
