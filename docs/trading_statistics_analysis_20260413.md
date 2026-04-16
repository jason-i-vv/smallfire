# 模拟交易统计分析模块优化

> 日期：2026-04-13
> 关联需求：模拟交易统计分析界面完善

## 一、背景

前端交易管理模块定位为"模拟交易统计分析"，但存在以下问题：
1. 权益曲线数据是伪造的（前端用平均分配模拟生成）
2. 后端已有统计指标前端未全量展示
3. 分析维度单一，缺少按标的、方向、出场原因、时间周期的细分分析
4. 信号分析粒度过粗（仅按 source_type 分组）
5. Dashboard 调用 `tradeApi.equity()` 但 API 层未定义

## 二、后端变更

### 2.1 新增 API 端点

| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/v1/trades/equity-curve` | GET | 权益曲线数据（真实累积资金） |
| `/api/v1/trades/symbol-analysis` | GET | 按标的分组统计 |
| `/api/v1/trades/direction-analysis` | GET | 按多/空方向分组统计 |
| `/api/v1/trades/exit-reason-analysis` | GET | 按出场原因分组统计 |
| `/api/v1/trades/period-pnl` | GET | 按日/周/月分组盈亏（period 参数） |
| `/api/v1/trades/pnl-distribution` | GET | 盈亏分布直方图数据 |
| `/api/v1/trades/signal-analysis-detail` | GET | 按具体信号类型分析（box_breakout 等） |

所有端点支持 `start_date` / `end_date` 查询参数进行日期筛选。

### 2.2 新增数据结构

- `EquityCurvePoint` — 权益曲线点（time + equity）
- `SymbolAnalysis` — 按标的统计（symbol_id, symbol_code, trades, win_rate, pnl）
- `DirectionAnalysis` — 按方向统计（direction, trades, win_rate, pnl, avg_holding_hours）
- `ExitReasonAnalysis` — 按出场原因统计（exit_reason, trades, win_rate, pnl）
- `PeriodPnL` — 按周期盈亏（period_start, pnl, trade_count）
- `PnLDistribution` / `PnLBucket` — 盈亏分布直方图

### 2.3 依赖注入更新

`StatisticsService` 新增 `symbolRepo` 依赖，用于查询标的代码。

## 三、前端变更

### 3.1 新增组件

| 组件 | 说明 |
|------|------|
| `EquityCurveChart.vue` | 权益曲线（lightweight-charts area series） |
| `PnLByPeriodChart.vue` | 日/周/月盈亏柱状图（histogram series） |
| `PnLDistributionChart.vue` | 盈亏分布（CSS 横向条形图） |
| `SymbolAnalysisTable.vue` | 按标的统计表格 |
| `DirectionAnalysisPanel.vue` | 多/空方向对比卡片 |
| `ExitReasonPanel.vue` | 出场原因分析面板 |
| `SignalAnalysisTable.vue` | 信号类型分析表格（按具体类型） |

### 3.2 页面布局

Statistics.vue 重写为 7 个区域：
1. 日期筛选栏
2. 12 项核心指标卡片（总收益率、总盈亏、胜率、盈亏比、最大回撤、交易次数、夏普比率、卡玛比率、平均盈利、平均亏损、期望值、平均持仓时间）
3. 权益曲线 + 日/周/月盈亏柱状图
4. 多/空方向分析 + 出场原因分析
5. 盈亏分布 + 信号类型分析
6. 按标的统计表格
7. 近期交易记录

### 3.3 Dashboard 修复

- `tradeApi.equity()` 已有对应 API 定义
- 适配新 API 返回格式 `[{time, equity}]`
- 适配 stats 返回格式

## 四、关键文件

- `internal/service/trading/statistics.go` — 7 个新服务方法
- `internal/handler/trade_handler.go` — 7 个新 handler
- `cmd/server/main.go` — 路由注册 + 依赖注入
- `starfire-frontend/src/api/trades.js` — 7 个新 API 方法
- `starfire-frontend/src/views/trades/Statistics.vue` — 页面重写
- `starfire-frontend/src/views/dashboard/Dashboard.vue` — equity 修复
- `starfire-frontend/src/components/charts/` — 3 个新图表组件
- `starfire-frontend/src/components/trades/` — 4 个新分析组件
