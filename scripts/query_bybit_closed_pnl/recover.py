#!/usr/bin/env python3
"""
一次性修复所有 testnet 异常持仓
1. 从数据库读取所有 anomalous 状态的 track（带 exchange_order_id）
2. 按 symbol 批量查 Bybit closed-pnl 历史
3. 用 order_id 精确匹配
4. 输出 UPDATE SQL 并可选择直接执行
"""
import hmac
import hashlib
import time
import requests
import subprocess
import sys
import os
import json

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


def call_bybit_all(path):
    """分页获取所有历史记录（最多5页=250条）"""
    all_items = []
    cursor = ""
    page = 1
    while True:
        p = path
        if cursor:
            p = f"{path}&cursor={cursor}"
        sig, ts = sign(p)
        headers = {
            "X-BAPI-API-KEY": API_KEY,
            "X-BAPI-TIMESTAMP": ts,
            "X-BAPI-SIGN": sig,
            "X-BAPI-RECV-WINDOW": RECV_WINDOW,
        }
        resp = requests.get(BASE_URL + p, headers=headers, timeout=30)
        data = resp.json()
        if data.get("retCode") != 0:
            print(f"    Bybit API error: {data.get('retMsg')}")
            break
        result = data.get("result", {})
        items = result.get("list", [])
        all_items.extend(items)
        cursor = result.get("nextPageCursor", "")
        if not cursor or page >= 5:
            break
        page += 1
        time.sleep(0.3)
    return all_items


def get_db_anomalous():
    """从数据库读取所有 anomalous 状态的 track"""
    cmd = [
        "sudo", "docker", "exec", "starfire-postgres",
        "psql", "-U", "postgres", "-d", "starfire_quant",
        "-t", "-A",
        "-c",
        "SELECT t.id, s.symbol_code, t.direction, t.entry_price, t.entry_time, t.quantity, t.position_value, COALESCE(t.exchange_order_id,'') FROM trade_tracks t JOIN symbols s ON t.symbol_id = s.id WHERE t.status = 'anomalous' AND t.exchange_order_id IS NOT NULL AND t.exchange_order_id != '' ORDER BY t.created_at"
    ]
    result = subprocess.run(cmd, capture_output=True, text=True)
    tracks = []
    for line in result.stdout.strip().split("\n"):
        if not line:
            continue
        parts = line.split("|")
        if len(parts) >= 8:
            tracks.append({
                "id": parts[0],
                "symbol": parts[1],
                "direction": parts[2],
                "entry_price": parts[3],
                "entry_time": parts[4],
                "quantity": parts[5],
                "position_value": parts[6],
                "order_id": parts[7],
            })
    return tracks


def infer_exit_reason(pnl_val, entry_price, exit_price, direction, stop_loss, take_profit):
    """根据价格判断平仓原因"""
    if exit_price and entry_price:
        ep = float(entry_price)
        xp = float(exit_price)
        if direction == "long":
            if stop_loss and xp <= float(stop_loss):
                return "stop_loss"
            if take_profit and xp >= float(take_profit):
                return "take_profit"
        else:
            if stop_loss and xp >= float(stop_loss):
                return "stop_loss"
            if take_profit and xp <= float(take_profit):
                return "take_profit"
    if pnl_val < 0:
        return "stop_loss"
    return "take_profit"


def main():
    dry_run = "--dry-run" in sys.argv
    if dry_run:
        print("=== DRY RUN 模式，只输出 SQL 不执行 ===\n")

    print("正在从数据库读取 anomalous 持仓...")
    tracks = get_db_anomalous()
    print(f"共 {len(tracks)} 条异常持仓\n")

    if not tracks:
        print("没有找到异常持仓")
        return

    # 按 symbol 分组
    sym_map = {}
    for t in tracks:
        sym = t["symbol"]
        if sym not in sym_map:
            sym_map[sym] = []
        sym_map[sym].append(t)

    print(f"涉及 {len(sym_map)} 个不同 symbol\n")

    # 用 order_id 集合来精确匹配
    order_ids = {t["order_id"]: t for t in tracks}

    # 按 symbol 查 Bybit，匹配 order_id
    for sym, sym_tracks in sym_map.items():
        print(f"=== {sym} ({len(sym_tracks)} 条) ===")
        items = call_bybit_all(f"/v5/position/closed-pnl?category=linear&symbol={sym}&limit=50")
        print(f"  Bybit 返回 {len(items)} 条历史记录")

        # 建立 track_id -> matched item 的映射
        track_matches = {}
        for item in items:
            oid = item.get("orderId", "")
            if oid in order_ids:
                t = order_ids[oid]
                track_matches[t["id"]] = (item, "order_id")

        # 对未匹配的 track，尝试用 entry_price + entry_time 窗口匹配（±1% 价格，±10分钟）
        for t in sym_tracks:
            tid = t["id"]
            if tid in track_matches:
                continue
            try:
                db_entry = float(t["entry_price"])
                # DB 存的是 UTC，strptime 解析后按本地时间转 timestamp，再转 UTC
                # 数据库时间格式: 2026-05-09 02:00:00+00
                db_time_str = t["entry_time"].replace("+00", "").strip()
                # time.mktime interprets as local time, so subtract local timezone offset
                # to get UTC timestamp. On UTC servers this is 0 offset.
                import calendar
                db_time_utc = calendar.timegm(time.strptime(db_time_str, "%Y-%m-%d %H:%M:%S"))
            except Exception as e:
                print(f"  [NOT FOUND] id={tid} order={t['order_id'][:8]}... entry_price={t['entry_price']} (解析时间失败: {e})\n")
                continue

            best = None
            best_score = float("inf")
            for item in items:
                try:
                    bp_entry = float(item.get("avgEntryPrice", 0))
                    if bp_entry <= 0:
                        continue
                    bp_time = int(str(item.get("createdTime", 0))) / 1000
                    entry_diff_pct = abs(bp_entry - db_entry) / db_entry * 100  # percentage
                    time_diff_sec = abs(bp_time - db_time_utc)
                    if entry_diff_pct < 2.0 and time_diff_sec < 7200:  # 2% 价格差，2小时内
                        score = entry_diff_pct * 10 + time_diff_sec / 3600  # price weighted more
                        if score < best_score:
                            best_score = score
                            best = item
                except:
                    continue
            if best:
                track_matches[tid] = (best, "price_time")
            else:
                print(f"  [NOT FOUND] id={tid} order={t['order_id'][:8]}... entry_price={t['entry_price']} db_time={db_time_str} 在 Bybit {len(items)} 条记录中未匹配\n")

        # 输出所有匹配结果
        for t in sym_tracks:
            tid = t["id"]
            if tid not in track_matches:
                continue
            item, match_type = track_matches[tid]
            entry = float(item.get("avgEntryPrice", 0))
            exit = float(item.get("avgExitPrice", 0))
            pnl = float(item.get("closedPnl", 0))
            qty = float(item.get("qty", 0))
            open_fee = float(item.get("openFee", 0))
            close_fee = float(item.get("closeFee", 0))
            total_fee = open_fee + close_fee
            occuring_time = int(str(item.get("createdTime", 0)))
            exit_ts = time.strftime("%Y-%m-%d %H:%M:%S", time.gmtime(occuring_time / 1000))
            exit_reason = infer_exit_reason(pnl, entry, exit, t["direction"], None, None)
            tag = "order_id" if match_type == "order_id" else "price_time"

            sql = (
                f"UPDATE trade_tracks SET "
                f"status='closed', "
                f"entry_price={entry:.8f}, "
                f"exit_price={exit:.8f}, "
                f"quantity={qty:.8f}, "
                f"pnl={pnl:.8f}, "
                f"fees={total_fee:.8f}, "
                f"exit_reason='{exit_reason}', "
                f"exit_time=TIMESTAMP '{exit_ts}', "
                f"updated_at=NOW() "
                f"WHERE id={tid};"
            )
            print(f"  [MATCHED:{tag}] id={tid} entry={entry} exit={exit} pnl={pnl} exit_reason={exit_reason}")
            print(f"  SQL: {sql}\n")

    print("\n=== 完成 ===")


if __name__ == "__main__":
    main()
