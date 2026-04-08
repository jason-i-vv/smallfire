# 模拟交易模块 Review 与修复记录

**日期**: 2026-04-08
**关联需求**: REQ-TRADING-001
**变更类型**: BUG修复 + 功能完善 + 测试覆盖

---

## 一、Review 发现的问题

### CRITICAL 级别

1. **手动平仓绕过 TradeExecutor** (`trade_handler.go:196-259`)
   - ClosePosition handler 直接操作 trackRepo，绕过了 TradeExecutor
   - 导致账户权益不更新（positionSizer.UpdateCapital 未调用）
   - ExitTime 设为零值 `&time.Time{}`
   - PnL 计算与 Executor 重复（DRY 违反）
   - 无通知发送

2. **零测试覆盖**
   - 整个 `internal/service/trading/` 目录无任何测试文件

### HIGH 级别

3. **SharpeRatio/CalmarRatio 硬编码为 0** (`statistics.go:161-163`)
4. **getSignalType 永远返回 "unknown"** (`statistics.go:211-214`)
5. **前端统计页 8 个指标全部硬编码 mock 数据** (`Statistics.vue`)
6. **前端交易历史页未实现** (`History.vue` 仅占位符)

### MEDIUM 级别

7. **`err.Error() == "no rows in result set"` 脆弱错误判断** (`trade_track_repo.go`)
8. **25 列 SELECT/Scan 重复 7 次** DRY 违反
9. **GetOpenPositions() 无排序**
10. **MonitorFactory/Notifier 接口使用 `interface{}`** 类型不安全
11. **StatisticsService 吞没错误**

---

## 二、修复内容

### Phase 1: 修复手动平仓 BUG

| 文件 | 变更 |
|------|------|
| `internal/handler/trade_handler.go` | TradeHandler 新增 executor 字段；ClosePosition 改为调用 `executor.CloseByManual()`；删除 50 行重复代码 |
| `cmd/server/main.go` | tradeExecutor 不再丢弃 `_`；NewTradeHandler 注入 executor |

### Phase 2: 完善统计分析

| 文件 | 变更 |
|------|------|
| `internal/service/trading/statistics.go` | 实现 SharpeRatio 和 CalmarRatio 计算；修复 getSignalType 通过 SignalRepo 查询；新增 SignalRepo 依赖注入；消除错误吞没 |
| `cmd/server/main.go` | NewStatisticsService 注入 signalRepo |

### Phase 3: 补充核心测试

| 文件 | 用例数 | 覆盖 |
|------|--------|------|
| `position_sizer_test.go` | 7 | 仓位计算、最大仓位截断、零距离边界、资金更新 |
| `stop_loss_test.go` | 8 | 多头/空头止损止盈计算、边界触发、nil 指针 |
| `risk_manager_test.go` | 7 | 6 项风控规则通过/拒绝、零值信号时间 |
| `statistics_test.go` | 9 | 空数据、全胜/全亏、连胜连败、最大回撤、夏普/卡玛、信号类型查询 |

**总计: 31 个测试用例，全部通过**

### Phase 4: 前端对接

| 文件 | 变更 |
|------|------|
| `starfire-frontend/src/views/trades/Statistics.vue` | 删除硬编码 mock，接入 `tradeApi.stats()` 和 `tradeApi.signalAnalysis()`；新增信号分析展示；添加加载/空状态 |
| `starfire-frontend/src/views/trades/History.vue` | 从占位符实现完整页面：分页、日期筛选、对接 `tradeApi.history()` |
| `starfire-frontend/src/views/trades/Positions.vue` | 删除硬编码指标，改为从持仓数据计算（总保证金、持仓均价）；删除 mock 降级；添加加载状态 |
| `starfire-frontend/src/api/trades.js` | 新增 `closed()`、`signalAnalysis()`、`detail()` 接口 |

### Phase 5: 代码质量改进

| 文件 | 变更 |
|------|------|
| `internal/repository/trade_track_repo.go` | 提取 `scanTradeTrack` / `scanTradeTracks` 辅助函数消除重复；`err.Error()` 改为 `errors.Is(err, pgx.ErrNoRows)`；GetOpenPositions 添加 `ORDER BY` |
| `internal/service/trading/dependency.go` | MonitorFactory/Notifier 接口的 `interface{}` 改为 `*models.TradeTrack` |

---

---

## 三、QA 验证

### QA 发现的额外问题

**BUG: 无交易数据时 `current_capital` 返回 0**
- `statistics.go:75-77`：空数据分支只设了 `InitialCapital`，遗漏 `CurrentCapital`
- 导致 `/trades/stats` 在无交易时返回 `current_capital: 0` 而非 `initial_capital: 100000`
- 修复：空数据分支增加 `stats.CurrentCapital = s.config.InitialCapital`

### 验证结果

| 检查项 | 状态 |
|--------|------|
| `go build ./cmd/server/` — 编译通过 | PASS |
| `go test ./internal/service/trading/...` — 31/31 测试通过 | PASS |
| `/api/v1/trades/stats` — 返回正确结构（含 current_capital=initial_capital） | PASS |
| `/api/v1/trades/signal-analysis` — 空数据返回 `{}` | PASS |
| `/api/v1/trades/positions` — 空数据返回 `null` | PASS |
| `/api/v1/trades/history` — 返回分页结构 `list/page/size/total` | PASS |
| `/api/v1/trades/closed` — 空数据返回 `null` | PASS |
| 前端 API baseURL `http://localhost:8080/api/v1` 与后端路由匹配 | PASS |
| 前端 Statistics.vue 从 API 获取真实数据（无 mock 降级） | PASS |
| 前端 History.vue 分页、日期筛选、字段映射 | PASS |
| 前端 Positions.vue 指标从持仓数据计算 | PASS |
