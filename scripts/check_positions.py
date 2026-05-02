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

# 查所有 USDT 本位持仓
data = bybit_get("/v5/position/list?category=linear&settleCoin=USDT&limit=100")
lst = data.get("result", {}).get("list", [])
open_pos = [p for p in lst if int(p.get("size", "0")) != 0]
print(f"Bybit size!=0: {len(open_pos)} 条")
for p in open_pos:
    print(f"  {p['symbol']} {p['side']} size={p['size']} entry={p.get('entryPrice','?')} unrealPnl={p.get('unrealisedPnl','0')}")

# 查数据库
r = subprocess.run(["sudo", "docker", "exec", "starfire-postgres",
    "psql", "-U", "postgres", "-d", "starfire_quant", "-t", "-c",
    "SELECT s.symbol_code, t.id, t.entry_price FROM trade_tracks t JOIN symbols s ON t.symbol_id=s.id WHERE t.trade_source='testnet' AND t.status='open' ORDER BY t.id;"],
    capture_output=True, text=True)
lines = [l.strip() for l in r.stdout.strip().split("\n") if l.strip()]
print(f"\n数据库 open: {len(lines)} 条")
db_symbols = set()
for line in lines:
    parts = line.split("|")
    if len(parts) >= 3:
        sym = parts[0].strip()
        db_symbols.add(sym)
        print(f"  {sym} id={parts[1].strip()} entry={parts[2].strip()}")

bybit_symbols = set(p["symbol"] for p in open_pos)
missing = db_symbols - bybit_symbols
extra = bybit_symbols - db_symbols

print(f"\n数据库有但 Bybit 没有 ({len(missing)}): {sorted(missing)}")
print(f"Bybit 有但数据库没有 ({len(extra)}): {sorted(extra)}")
