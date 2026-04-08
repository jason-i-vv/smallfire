# K线数据完整性校验优化

## 日期
2026-04-07

## 背景

K线查询接口 `/api/v1/klines` 返回数据不完整（例如 24 小时 15m 周期应返回 96 根 K 线，实际只返回 18 根）。原因是行情抓取时 API 返回了不完整数据，但代码直接入库了。一旦错误数据入库，`GetLatest` 会以最新的不完整记录作为锚点，后续同步无法补回缺失的 K 线。

## 核心原则

**数据完整性不足时，不入库。让下次同步重新抓取。**

## 改动内容

### 1. sync_service.go — 新增数据完整性校验

在 `syncSymbolKlines` 入库前增加 `validateKlineData` 校验步骤：

**连续性检查**：
- 相邻 K 线的时间间隔不应超过 `periodDuration * 3`
- 捕获 API 返回数据中间有断档的情况

**回补完整率检查**（仅回补场景生效）：
- 根据回补时间范围计算期望 K 线数量
- 实际获取数量 < 期望数量的 50% 时，拒绝入库
- 例如：回补 24 小时的 15m 数据，期望约 96 条，实际 < 48 条则跳过

校验不通过时：
- 记录 WARN 级别日志（含 symbol、period、条数、首尾时间、失败原因）
- 返回 error，不计入成功数
- 数据不入库，下次同步会重新尝试

### 2. kline_repo.go — BatchCreate 增加 ON CONFLICT DO NOTHING

```sql
INSERT INTO klines (...) VALUES (...)
ON CONFLICT (symbol_id, period, open_time) DO NOTHING
```

- 防止因重复数据（如同步服务中途崩溃重启后重试）导致整个批次事务回滚
- 跳过已存在的 K 线，继续插入剩余数据
- 统计实际插入条数

## 涉及文件

| 文件 | 改动 |
|------|------|
| `internal/service/market/sync_service.go` | 新增 `validateKlineData` 方法，在入库前调用校验 |
| `internal/repository/kline_repo.go` | `BatchCreate` 添加 `ON CONFLICT DO NOTHING`，统计实际插入数 |

## 日志示例

校验通过时正常入库，校验不通过时日志如下：

```
WARN  K线数据完整性校验未通过，跳过入库等待下次重试
  symbol=ETHUSDT  period=15m  kline_count=18  is_backfill=true
  first_time=2026-04-05 01:30:00 UTC  last_time=2026-04-05 10:30:00 UTC
  reason=回补数据不完整: 时间跨度 24h0m0s，期望约 96 条，实际 18 条，完整率 18.8%（低于50%阈值）
```
