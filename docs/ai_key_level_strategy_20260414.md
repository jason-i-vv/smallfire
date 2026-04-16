# AI 关键价位识别策略

> 日期: 2026-04-14
> 状态: 已实现

## 需求背景

原有 KeyLevelStrategy 使用 swing point 算法识别支撑阻力位，存在以下问题：

1. **Swing point lookback=3 太敏感** - 15m 周期上，45分钟的微小波动就被识别为"阻力位"
2. **突破阈值 0.2% 太小** - BTC 价格 80000 时，涨 160 块就算突破，假突破频繁
3. **没有量能确认** - 无量突破最常见的结局就是假突破回落
4. **回溯窗口只有 50 根K线** - 15m 周期只看 12.5 小时，遗漏中长期关键价位
5. **无法识别心理价位** - 整数关口、历史高低点等对交易行为有重要影响

## 方案设计

采用**混合模式**：AI 负责识别关键价位（低频），算法负责监控突破（高频）。

```
AI Key Level Analyzer (低频，每4小时)     KeyLevelStrategy (高频，每根K线)
        │                                          │
        │ 1. 收集200根K线+EMA+成交量               │ 1. Swing point 检测(算法)
        │ 2. 调用AI识别关键支撑阻力                 │ 2. 读取所有活跃价位(algo+ai)
        │ 3. 写入key_levels(source=ai)              │ 3. 突破检测+量能确认
        │                                          │ 4. 生成信号
        └────────────┬─────────────────────────────┘
                     │
                     ▼
            opportunity_aggregator (已有)
```

### AI 价位识别器

- **触发频率**: 每 4 小时一次（可配置）
- **输入**: 200 根K线 OHLCV + EMA + 成交量 + 现有活跃价位
- **输出**: JSON 格式的支撑阻力列表（含强度评分）
- **去重**: AI 价位与算法价位在 0.3% 以内视为重复，不重复创建
- **过期**: 每次识别前，先过期该标的+周期的所有旧 AI 价位

### AI Prompt 设计

**System Prompt**:
- 要求识别真正的关键价位，不是每个小波动
- strength 评分标准：1-3(弱)，4-6(中)，7-10(强)
- source_type 分类：round_number, multi_test, swing_point, volume_cluster, historical
- 每侧最多 5 个价位
- 优先识别距当前价格较近的价位

**User Prompt**:
- 标的、周期、当前价
- 最近 20 根K线 OHLCV
- EMA30/60/90 趋势方向
- 20 周期均量
- 近期区间统计
- 现有活跃价位列表

### 突破监控优化

| 参数 | 修改前 | 修改后 | 原因 |
|------|--------|--------|------|
| lookback_klines | 50 | 200 | 捕获中长期价位 |
| level_distance | 0.2% | 0.5% | 减少假突破 |
| 去重容差 | 0.1% | 0.3% | 与触及容差一致 |
| 量能确认 | 无 | 突破K线量>均量*1.5 | 确认突破有效性 |
| AI价位成熟期 | N/A | 跳过minBreakoutAge | AI已评估强度 |

### 数据库变更

`key_levels` 表新增 3 列:
- `source VARCHAR(20) DEFAULT 'algorithm'` — 价位来源
- `strength INTEGER DEFAULT 1` — 强度评分
- `ai_reason TEXT` — AI 识别理由

## 配置说明

```yaml
# config.yml
ai:
  key_level:
    enabled: true              # 是否启用
    interval_minutes: 240      # 分析间隔（分钟）
    max_daily_calls: 200       # 每日最大调用次数
    cooldown_minutes: 60       # 同一标的冷却时间
    kline_count: 200           # 输入K线数量
    max_levels_per_side: 5     # 每侧最多价位数

strategies:
  key_level:
    enabled: true
    lookback_klines: 200       # 回溯K线数（已调大）
    level_distance: 0.5        # 突破阈值%（已调大）
    min_breakout_age: 5
    check_interval: 60
```

## 文件变更清单

| 文件 | 操作 |
|------|------|
| `internal/database/migrations/000011_key_level_ai_source.up.sql` | 新建 |
| `internal/models/key_level.go` | 修改 |
| `internal/repository/repository.go` | 修改 |
| `internal/repository/key_level_repo.go` | 修改 |
| `internal/service/ai/key_level_analyzer.go` | 新建 |
| `internal/config/config.go` | 修改 |
| `config/config.yml` | 修改 |
| `internal/service/strategy/key_level_strategy.go` | 修改 |
| `cmd/server/main.go` | 修改 |
| `docs/ai_key_level_strategy_20260414.md` | 新建 |

## 验证方法

1. `make backend` 启动后端
2. 观察日志确认 `AI关键价位识别器启动`
3. 检查 `key_levels` 表确认 AI 价位写入（source='ai'）
4. 观察信号生成，确认量能确认生效
5. 对比修改前后的假突破率
