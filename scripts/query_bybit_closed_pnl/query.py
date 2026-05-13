#!/usr/bin/env python3
import hmac
import hashlib
import time
import requests
import sys

API_KEY = "DatS4UH1Leyg9qG2Kc"
API_SECRET = "UlUKA1qDeVRa3XYAuzDbGVyWqZZICWvvmY6y"
BASE_URL = "https://api-demo.bybit.com"
RECV_WINDOW = "5000"


def sign(path_with_query):
    parts = path_with_query.split("?", 1)
    param_str = parts[1]
    ts = str(int(time.time() * 1000))
    payload = f"{ts}{API_KEY}{RECV_WINDOW}{param_str}"
    mac = hmac.new(API_SECRET.encode(), payload.encode(), hashlib.sha256)
    return mac.hexdigest(), ts


def call_bybit(path):
    sig, ts = sign(path)
    headers = {
        "X-BAPI-API-KEY": API_KEY,
        "X-BAPI-TIMESTAMP": ts,
        "X-BAPI-SIGN": sig,
        "X-BAPI-RECV-WINDOW": RECV_WINDOW,
    }
    resp = requests.get(BASE_URL + path, headers=headers, timeout=30)
    data = resp.json()
    if data.get("retCode") != 0:
        print(f"  Bybit API error: {data.get('retMsg')}")
        return [], ""
    result = data.get("result", {})
    return result.get("list", []), result.get("nextPageCursor", "")


def main():
    symbols = sys.argv[1:]
    if not symbols:
        print("Usage: python3 query.py SYMBOL [SYMBOL...]")
        print("Example: python3 query.py BTCUSDT ETHUSDT BSVUSDT")
        return

    for sym in symbols:
        print(f"\n=== {sym} ===")
        path = f"/v5/position/closed-pnl?category=linear&symbol={sym}&limit=50"
        items, cursor = call_bybit(path)
        print(f"Page 1: {len(items)} records")

        for item in items:
            created_ts = int(str(item.get("createdTime", 0)))
            ts = time.strftime("%Y-%m-%d %H:%M:%S", time.gmtime(created_ts / 1000))
            print(f"  [{ts}] orderID={item.get('orderId','?')} side={item.get('side','?')} "
                  f"qty={item.get('qty','?')} entry={item.get('avgEntryPrice','?')} "
                  f"exit={item.get('avgExitPrice','?')} pnl={item.get('closedPnl','?')} "
                  f"openFee={item.get('openFee','?')} closeFee={item.get('closeFee','?')}")

        page = 2
        while cursor and page <= 5:
            time.sleep(0.3)
            path2 = f"/v5/position/closed-pnl?category=linear&symbol={sym}&limit=50&cursor={cursor}"
            items2, cursor = call_bybit(path2)
            print(f"Page {page}: {len(items2)} records")
            for item in items2:
                created_ts = int(str(item.get("createdTime", 0)))
                ts = time.strftime("%Y-%m-%d %H:%M:%S", time.gmtime(created_ts / 1000))
                print(f"  [{ts}] orderID={item.get('orderId','?')} side={item.get('side','?')} "
                      f"qty={item.get('qty','?')} entry={item.get('avgEntryPrice','?')} "
                      f"exit={item.get('avgExitPrice','?')} pnl={item.get('closedPnl','?')} "
                      f"openFee={item.get('openFee','?')} closeFee={item.get('closeFee','?')}")
            page += 1


if __name__ == "__main__":
    main()
