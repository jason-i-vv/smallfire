# K 线数据时区修复 & 缺失数据回补

**日期**: 2026-04-03  
**影响范围**: 所有 K 线数据的 open_time / close_time 字段，以及同步服务的回补逻辑

## 问题描述

### 问题 1：时区错位

`open_time` 和 `close_time` 存储的是 UTC+8 本地时间，但 API 返回时带 `Z` 后缀（表示 UTC），导致前端显示的时间比实际快 8 小时。

**根因**：
- `time.Unix(timestamp, 0)` 返回 Go 本地时区（CST/UTC+8）的 `time.Time`
- pgx v5 将 `time.Time` 写入 `timestamp without time zone` 列时，使用 time 的 location 生成字面值
- 结果：Bybit 返回 15:00 UTC 的 K 线，被存储为 `2026-04-03 23:00:00`（UTC+8 表示），API 序列化为 `2026-04-03T23:00:00Z`

### 问题 2：K 线数据不连续

同步服务停机期间（多次 8-9 小时）产生的 K 线缺口未被回补。

**根因**：
- 断点续传逻辑只取最近 100 条 K 线，但 closed filter 存在 bug（检查了数组错误端）
- 没有主动检测缺口并使用 `FetchKlinesByTimeRange` 回补

## 修复方案

### 代码修复

| 文件 | 修改内容 |
|------|----------|
| `internal/service/market/bybit_fetcher.go` | `parseTimestamp`: `time.Unix().UTC()` 确保返回 UTC |
| `internal/service/market/eastmoney_fetcher.go` | `parseEastmoneyKlines`: `openTime.UTC()` 转为 UTC；`getPeriodEndTime`: 用 CST 计算收盘时间后 `.UTC()` |
| `internal/service/market/sina_fetcher.go` | `ParseInLocation` 后 `openTime.UTC()` |
| `internal/service/market/fetcher.go` | `convertToModel`: `time.Now().UTC()` |
| `internal/service/market/sync_service.go` | 1. 修复 closed filter：检查 `i==0`（最新条）而非 `i==len-1`（最旧条）<br>2. 新增缺口检测：计算 DB 最新记录与当前时间的差距，超过 1 小时则用 `FetchKlinesByTimeRange` 回补<br>3. EMA 计算前按 `open_time` 升序排序<br>4. `time.Now().UTC()` 统一时区 |

### 数据迁移

**脚本**: `db-scripts/fix_kline_timezone_and_backfill.sql`

```sql
-- 所有 klines 的 open_time/close_time 减 8 小时
ALTER TABLE klines DROP CONSTRAINT klines_symbol_id_period_open_time_key;
UPDATE klines SET open_time = open_time - INTERVAL '8 hours', close_time = close_time - INTERVAL '8 hours';
ALTER TABLE klines ADD CONSTRAINT klines_symbol_id_period_open_time_key UNIQUE (symbol_id, period, open_time);
```

**注意**: 执行前需备份数据，并停止同步服务避免写入冲突。

### 回补机制

同步服务重启后会自动检测缺口并回补：
- 最大回补范围：168 小时（7 天）
- 使用 `FetchKlinesByTimeRange` 按时间范围拉取数据
- 仅回补已收盘（`close_time <= now`）的 K 线

## 验证

修复后 DB 数据与 Bybit API 完全一致：
- DB `2026-04-03T08:00:00Z open=0.006434` = Bybit `08:00 UTC open=0.006434`

---

## 2026-04-06 补充修复

### 问题 1：信号图表时间显示为 UTC

`KlineChart.vue` 的 `timeFormatter` 使用 `getUTCHours()` 等方法显示 UTC 时间，与回测图表（UTC+8）不一致。

**修复**: 将时间戳加 8 小时偏移后使用 UTC 方法提取时间组件，确保显示 UTC+8 时间。

| 文件 | 修改内容 |
|------|----------|
| `starfire-frontend/src/views/kline/KlineChart.vue` | `timeFormatter`: `new Date(timestamp * 1000 + 8 * 3600 * 1000)` 偏移 +8h 后再用 `getUTCHours()` 提取 |
| `starfire-frontend/src/views/kline/KlineChart.vue` | `fetchKlines`: 信号模式下前后各获取 50 根 K 线（原为 100 根），并支持从信号详情自动获取信号时间 |
| `starfire-frontend/src/views/kline/KlineChart.vue` | `fetchKlines`: 当有 `signalId` 但无 `signalTime` 时，先通过 `signalApi.detail()` 获取信号时间 |

### 问题 2：未收盘 K 线触发策略分析

`runner.go` 的 `analyzeAllSymbols()` 获取 K 线后未过滤未收盘的 K 线，导致当前正在形成中的 K 线也参与策略分析，产生错误信号。

**修复**: 在 K 线反转排序后，过滤掉 `CloseTime` 在当前时间之后的 K 线。

| 文件 | 修改内容 |
|------|----------|
| `internal/service/strategy/runner.go` | `analyzeAllSymbols`: 排序后遍历 klines，跳过 `CloseTime.After(now)` 的未收盘 K 线 |

**逻辑**:
```go
now := time.Now()
filtered := make([]models.Kline, 0, closedCount)
for _, k := range klines {
    if !k.CloseTime.After(now) {
        filtered = append(filtered, k)
    }
}
klines = filtered
```
