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

# 查所有持仓
data = bybit_get("/v5/position/list?category=linear&limit=100")
lst = data["result"]["list"]
open_pos = [p for p in lst if int(p.get("size","0")) != 0]
print(f"Bybit 共 {len(lst)} 条记录，其中 size!=0 的有 {len(open_pos)} 条")
for p in open_pos:
    print(f"  {p['symbol']} {p['side']} size={p['size']} entry={p['entryPrice']} unrealizedPnl={p.get('unrealisedPnl','0')}")

# 查数据库 open 数量
r = subprocess.run(["sudo", "docker", "exec", "starfire-postgres",
    "psql", "-U", "postgres", "-d", "starfire_quant", "-t", "-c",
    "SELECT COUNT(*) FROM trade_tracks WHERE trade_source = 'testnet' AND status = 'open';"],
    capture_output=True, text=True)
print(f"\n数据库 open 记录数: {r.stdout.strip()}")

# 差值
db_open = int(r.stdout.strip())
diff = db_open - len(open_pos)
print(f"差值: 数据库多 {diff} 条")

# 列出数据库 open 但 bybit 没有的交易对
db_symbols = set()
for line in subprocess.run(["sudo", "docker", "exec", "starfire-postgres",
    "psql", "-U", "postgres", "-d", "starfire_quant", "-t", "-c",
    "SELECT s.symbol_code FROM trade_tracks t JOIN symbols s ON t.symbol_id=s.id WHERE t.trade_source='testnet' AND t.status='open';"],
    capture_output=True, text=True).stdout.strip().split("\n"):
    if line.strip():
        db_symbols.add(line.strip())

bybit_symbols = set(p["symbol"] for p in open_pos)
missing = db_symbols - bybit_symbols
extra = bybit_symbols - db_symbols

print(f"\n数据库有但 Bybit 没有的交易对 ({len(missing)}):")
for s in sorted(missing):
    print(f"  {s}")

print(f"\nBybit 有但数据库没有的交易对 ({len(extra)}):")
for s in sorted(extra):
    print(f"  {s}")
