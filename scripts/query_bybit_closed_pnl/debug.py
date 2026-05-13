#!/usr/bin/env python3
import hmac
import hashlib
import time
import requests
import calendar

API_KEY = "DatS4UH1Leyg9qG2Kc"
API_SECRET = "UlUKA1qDeVRa3XYAuzDbGVyWqZZICWvvmY6y"
BASE_URL = "https://api-demo.bybit.com"


def sign(path_with_query):
    parts = path_with_query.split("?", 1)
    param_str = parts[1]
    ts = str(int(time.time() * 1000))
    payload = f"{ts}{API_KEY}{5000}{param_str}"
    mac = hmac.new(API_SECRET.encode(), payload.encode(), hashlib.sha256)
    return mac.hexdigest(), ts


def call_bybit(path):
    sig, ts = sign(path)
    headers = {
        "X-BAPI-API-KEY": API_KEY,
        "X-BAPI-TIMESTAMP": ts,
        "X-BAPI-SIGN": sig,
        "X-BAPI-RECV-WINDOW": "5000",
    }
    resp = requests.get(BASE_URL + path, headers=headers, timeout=30)
    data = resp.json()
    if data.get("retCode") != 0:
        return [], data.get("retMsg"), ""
    result = data.get("result", {})
    return result.get("list", []), "", result.get("nextPageCursor", "")


# DB data for NOT FOUND positions
test_cases = [
    ("SOLVUSDT", 0.00452700, "2026-05-10 01:00:00+00"),
    ("CETUSUSDT", 0.03361700, "2026-05-10 02:00:00+00"),
    ("LITUSDT", 1.08020000, "2026-05-10 20:00:00+00"),
]

for sym, db_entry_price, db_entry_time in test_cases:
    print(f"\n=== {sym} ===")
    items, err, cursor = call_bybit(
        f"/v5/position/closed-pnl?category=linear&symbol={sym}&limit=50"
    )
    if err:
        print(f"Error: {err}")
        continue

    print(f"Records: {len(items)}, cursor: {cursor!r}")

    # Parse DB entry time to UTC timestamp using calendar.timegm (treats as UTC)
    db_time_str = db_entry_time.replace("+00", "").strip()
    db_ts = calendar.timegm(time.strptime(db_time_str, "%Y-%m-%d %H:%M:%S"))

    db_time_gm = time.strftime("%Y-%m-%d %H:%M:%S", time.gmtime(db_ts))
    print(f"DB: entry={db_entry_price} UTC_ts={db_ts} (= {db_time_gm} UTC)")

    if not items:
        print("  (no records at all)")
        continue

    for it in items:
        be = float(it.get("avgEntryPrice", 0))
        bt = int(str(it.get("createdTime", 0))) / 1000
        dp = abs(be - db_entry_price) / db_entry_price * 100
        ds = abs(bt - db_ts)
        m = dp < 1.0 and ds < 600
        bt_gm = time.strftime("%Y-%m-%d %H:%M:%S", time.gmtime(bt))
        print(
            f"  Bybit: entry={be} ts={bt:.0f} ({bt_gm} UTC) diff_pct={dp:.3f}% "
            f"diff_sec={ds:.0f}s match={m}"
        )
