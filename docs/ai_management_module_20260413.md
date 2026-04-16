# AI 分析管理模块

> 日期：2026-04-13
> 关联需求：AI 调用统计分析

## 一、背景

需要对 AI 调用情况进行全面统计分析：每日调用次数、AI 评估意见分布、AI 准确率、AI 判断与交易盈亏的关联。

核心问题是 `trade_tracks` 表缺少与 `trading_opportunities` 的关联，AutoTrader 从 Opportunity 创建交易时不记录来源。

## 二、数据库变更

新增迁移 `000010_trade_track_opportunity_id`：
- `trade_tracks` 新增 `opportunity_id` 字段（nullable FK → trading_opportunities）
- 创建条件索引

## 三、后端变更

### 3.1 Model 更新
- `TradeTrack` 新增 `OpportunityID *int` 字段
- `AutoTrader.openFixedPosition` 设置 `OpportunityID: &opp.ID`

### 3.2 AI 统计服务（`internal/service/ai/stats_service.go`）

新增 `AIStatsService`，直接使用 `*database.DB` 执行 SQL：

| 方法 | 说明 |
|------|------|
| `GetDailyCallStats` | 每日调用次数 |
| `GetOverview` | 总调用、平均置信度、一致率、方向分布 |
| `GetAccuracyAnalysis` | AI 准确率（一致胜率、分歧正确率、综合准确率） |
| `GetDirectionStats` | 按方向（多/空/中性）统计调用数、胜率、盈亏 |
| `GetConfidenceAnalysis` | 高(≥70)/中(40-69)/低(<40) 置信度分桶分析 |

### 3.3 API 端点

```
GET /api/v1/ai-stats/daily       → 每日调用统计
GET /api/v1/ai-stats/overview    → AI 概览
GET /api/v1/ai-stats/accuracy    → 准确率分析
GET /api/v1/ai-stats/direction   → 方向统计
GET /api/v1/ai-stats/confidence  → 置信度分析
```

均支持 `start_date` / `end_date` 查询参数。

## 四、前端变更

### 4.1 新增文件

| 文件 | 说明 |
|------|------|
| `src/api/ai-stats.js` | AI 统计 API 客户端 |
| `src/views/ai/AIManagement.vue` | AI 管理页面 |
| `src/components/charts/DailyCallChart.vue` | 每日调用柱状图 |
| `src/components/ai/AccuracyPanel.vue` | 准确率分析面板 |
| `src/components/ai/ConfidencePanel.vue` | 置信度分析面板 |
| `src/components/ai/DirectionStatsTable.vue` | 方向统计表格 |

### 4.2 页面布局

1. 日期筛选栏
2. 6 项概览卡片（总调用、平均信心、一致率、AI 准确率、一致胜率、分歧正确率）
3. 每日调用柱状图 + AI 方向分布
4. AI 准确率分析面板（一致/分歧/综合三栏对比）
5. 置信度分析面板（高/中/低三档）
6. AI 方向详细统计表格

### 4.3 路由和导航

- 路由：`/ai-management` → AIManagement
- 侧边栏新增"AI 管理"菜单项（Cpu 图标）

## 五、关键设计决策

- **opportunity_id 关联**：新增 nullable FK 到 trade_tracks，历史数据为 NULL 不受影响
- **准确率定义**：一致+盈利 = AI 正确；分歧+亏损 = AI 正确（AI 看对了反向）
- **置信度分档**：高(≥70)、中(40-69)、低(<40)
- **pgx JSONB 处理**：pgx 二进制协议不能直接将 JSONB scan 到 `map[string]interface{}`（`models.JSONB`），需在 SQL 层用 `->>` 运算符提取字段后再 scan 到基本类型
- **PostgreSQL date 类型**：pgx 二进制协议不能将 `date` 类型 scan 到 `*string`，需在 SQL 中用 `::text` 转换

## 六、关键文件清单

- `internal/database/migrations/000010_trade_track_opportunity_id.up.sql`
- `internal/models/trade_track.go`
- `internal/service/trading/auto_trader.go`
- `internal/service/ai/stats_service.go`
- `internal/handler/ai_stats_handler.go`
- `cmd/server/main.go`
- `starfire-frontend/src/api/ai-stats.js`
- `starfire-frontend/src/views/ai/AIManagement.vue`
- `starfire-frontend/src/components/ai/`
- `starfire-frontend/src/components/charts/DailyCallChart.vue`
- `starfire-frontend/src/components/common/AppSidebar.vue`
- `starfire-frontend/src/router/index.js`
