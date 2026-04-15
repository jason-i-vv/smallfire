# 策略优化、AI分析与交易机会系统整合

**日期:** 2026-04-15
**版本:** v0.1.0.0
**分支:** feat/strategy-optimization

## 变更概述

本次变更是一个大规模的功能迭代，涵盖策略模块重构、AI分析系统集成、交易机会自动创建、前端页面增强等多个模块。

## 策略模块优化

### Wick策略变更
- **原逻辑:** 趋势不匹配时直接返回nil，不产生信号（导致牛市中只有做空信号）
- **新逻辑:** 趋势不匹配时仍产生信号，但降低强度评分（基础强度-1）
- 影响: 信号数量增加，但质量通过强度评分过滤

### 关键位策略增强
- 增加量能确认逻辑
- 关键位V2数据模型（AI来源标识）
- 关键位AI分析器集成

### 趋势策略优化
- 趋势计算和策略执行分离
- 增强回撤确认逻辑

## AI分析系统

### 新增模块
- `internal/service/ai/analyzer.go` — 市场分析器
- `internal/service/ai/key_level_analyzer.go` — 关键位AI分析
- `internal/service/ai/client.go` — AI客户端
- `internal/service/ai/cooldown.go` — 调用频率冷却管理
- `internal/service/ai/stats_service.go` — AI统计服务

### 安全措施
- LLM输出价格范围验证 (0, 1e9)
- 文本长度限制 (Reason: 500字, Reasoning: 2000字)
- 条目数量限制 (最大10条)
- 枚举值校验 (Strength, Direction)

## 交易机会系统

### 核心组件
- `internal/service/scoring/signal_scorer.go` — 信号评分器
- `internal/service/scoring/opportunity_aggregator.go` — 机会聚合器
- `internal/service/trading/auto_trader.go` — 自动交易器

### 流程
1. 策略产生信号 → 2. 聚合器按币对+方向分组 → 3. 评分器计算总分 → 4. 创建/更新交易机会 → 5. 触发通知和自动交易回调

### 并发安全
- 聚合器使用mutex防止find-or-create竞态
- 统计更新使用UPSERT (ON CONFLICT) 原子操作

## 前端增强

### 新增页面
- AI管理 (`/ai`) — AI分析统计和管理
- 市场总览 (`/market`) — 市场概况
- 交易机会 (`/opportunities`) — 机会列表

### 组件增强
- K线图组件 (577行变更) — 增强标注和交互
- 交易统计页面重写 — 新增权益曲线、PnL分布等图表
- AI分析对话框 — 在K线图中展示AI判断结果

## 数据库迁移
- `000008_trading_opportunity` — 交易机会表
- `000009_trade_track_nullable_signal_id` — 交易追踪信号ID可空
- `000010_trade_track_opportunity_id` — 交易追踪关联机会ID
- `000011_key_level_ai_source` — 关键位AI来源标识
- `000012_key_levels_v2` — 关键位V2表

## 安全修复
1. SQL参数化LIMIT/OFFSET查询
2. 统计更新改用UPSERT避免TOCTOU竞态
3. 聚合器mutex防并发
4. opp.Signals nil评分逻辑修复
5. 未认证用户角色从admin降为viewer
6. LLM输出验证增强

## 配置变更
- `config/config.yml` — 新增AI和评分相关配置项
- `.gitignore` — 新增 `starfire-frontend/dist/` 和 `server`
