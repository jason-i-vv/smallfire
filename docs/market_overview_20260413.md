# 行情界面需求文档

日期: 2026-04-13

## 需求描述

实现一个行情界面，可以查看当前数据库的所有标的，在 K 线图上切换不同周期，显示趋势状态、EMA 均线和支撑阻力位置。

## 功能清单

### 1. 行情总览页 (/market)

- 市场切换 tab (Bybit / A股 / 美股)
- 标的列表表格: 代码、名称、最新价、涨跌幅、趋势状态、趋势强度
- 涨跌幅颜色编码: 正值绿色、负值红色
- 趋势标签: 看多(success)、看空(danger)、震荡(info)
- 分页 (每页 20 条)
- 点击行跳转 K 线图

### 2. K 线图增强 (/chart/:symbol)

- 周期切换按钮 (15m / 1h / 1d)
- 标的搜索选择器 (全市场搜索)
- 趋势状态栏 (当前周期趋势方向 + 强度)
- EMA 均线始终显示 (EMA30/60/90)
- 支撑阻力位始终显示

### 3. 后端 API

- `GET /api/v1/symbols/:id/trends?period=15m` - 获取标的趋势状态
- `GET /api/v1/markets/:code/overview?page=1&page_size=20` - 市场总览(标的+价格+趋势)

## 涉及文件

### 新增
- `internal/handler/trend_handler.go`
- `starfire-frontend/src/views/market/MarketOverview.vue`
- `starfire-frontend/src/api/markets.js`
- `starfire-frontend/src/api/trends.js`

### 修改
- `internal/handler/market_handler.go` - 新增 overview endpoint
- `cmd/server/main.go` - 注册新 handler 和路由
- `starfire-frontend/src/views/kline/KlineChart.vue` - 周期切换 + 标的选择器 + 始终显示 EMA/Key Levels
- `starfire-frontend/src/router/index.js` - 新增 /market 路由
- `starfire-frontend/src/components/common/AppSidebar.vue` - 添加行情总览菜单项

## 技术决策

1. Overview API 使用分页加载 (每页 20 条, 41 queries per page)
2. KlineChart 抽取周期选择器、标的选择器为独立小组件
3. EMA 和支撑阻力始终显示，不按信号类型条件判断
4. 周期切换使用内部状态驱动 (watch(period) 触发数据重载)
5. 标的选择器支持全市场搜索
6. 桌面优先，最小宽度 1024px
