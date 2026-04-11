# K线数据显示优化

## 日期：2026-04-09

## 背景

K线图表页面（`/chart/:symbol`）显示K线数量过少，原因有三：
1. 前端信号/箱体模式时间窗口仅 ±50 根K线（约 ±12.5 小时 @15m 周期）
2. 数据库历史数据不足时，后端直接返回少量数据，无API补充机制
3. 新跟踪标的首次同步只拉取 100 根K线，无法覆盖 ±200 窗口需求

## 变更内容

### 1. 前端信号模式窗口扩展（±50 → ±200）

**文件**：`starfire-frontend/src/views/kline/KlineChart.vue`

- 新增常量 `SIGNAL_CONTEXT_BARS = 200`
- 替换 4 处硬编码的 `50`：箱体模式时间范围、信号模式时间范围、scrollToTime 可见范围
- 对于 15m 周期，时间窗口从 ±12.5 小时扩大到 ±50 小时

### 2. DB数据不足时从交易所API补充

**文件**：
- `internal/service/market/kline_service.go` — 完善 GetKlines() API回退逻辑
- `internal/handler/symbol_handler.go` — GetKlines/GetSymbolKlines 改用 KlineService
- `cmd/server/main.go` — KlineService 注入 symbolRepo，Handler 注入 klineService

**逻辑流程**：
1. 查询本地数据库
2. 判断数据是否不足（有明确时间范围：实际数量 < 期望×70%；无时间范围：实际 < limit）
3. 通过 `factory.GetFetcher()` 获取对应交易所 fetcher
4. 调用 `FetchKlinesByTimeRange()` 从交易所API获取数据
5. API数据 `BatchCreate` 入库（ON CONFLICT DO NOTHING 幂等）
6. 入库成功后重新查DB返回（unique constraint 保证去重）
7. API失败时降级返回已有DB数据，不阻断请求

**关键设计**：
- `KlineService` 已有 `factory` 但 GetKlines() 的回退逻辑是 TODO 状态，本次完成实现
- 入库后再查DB而非手动合并，由数据库保证数据一致性
- DB 数据优先（可能有计算过的 EMA 值）

### 3. 同步服务新标的首次拉取增加到 500 根

**文件**：`internal/service/market/sync_service.go`

- 新增常量 `initialSyncLimit = 500`，`regularSyncLimit = 100`
- `syncSymbolKlines()` 中 `latestKline == nil` 时使用 `initialSyncLimit`
- 与现有 `validateKlineData()` 兼容（首次同步 isBackfill=false，不触发50%完整率校验）
