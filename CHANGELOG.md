# Changelog

All notable changes to this project will be documented in this file.

## [0.1.0.0] - 2026-04-15

### Added
- AI 分析模块: 关键位AI分析器、市场分析器、冷却管理、统计服务
- 交易机会系统: 信号聚合评分、机会自动创建和管理
- 自动交易模块: 基于评分的自动开仓和K线价格提供器
- 前端AI管理页面、市场总览页面、交易机会列表页面
- 前端AI分析对话框、准确率面板、置信度面板、方向统计表
- 前端权益曲线图、PnL分布图、PnL周期图、每日调用图
- 新增数据库迁移: 交易机会、关键位V2、AI来源等
- 趋势分析API端点和前端路由

### Changed
- 策略模块重构: 关键位策略和趋势策略大幅优化，增强量能确认
- Wick策略: 趋势不再硬性过滤信号，改为影响强度评分
- K线图组件大幅增强(577行变更)
- 交易统计模块和前端页面重写
- 侧边栏和路由更新，支持新页面
- 飞书通知增强
- 清理: 移除dist和server binary追踪，加入gitignore

### Fixed
- SQL参数化LIMIT/OFFSET查询，统计更新改用UPSERT避免竞态
- 交易机会聚合器并发竞态防护，Signals nil评分逻辑修复
- LLM输出增加价格范围/长度/数量验证
- 未认证用户角色从admin降为viewer
- WickStrategy趋势过滤测试更新匹配新逻辑
