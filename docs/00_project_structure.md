# 星火量化系统 - 项目结构设计

## 1. 后端项目结构 (Go)

```
starfire/                          # 后端项目根目录
├── cmd/                           # 应用程序入口
│   └── server/                    # 主服务入口
│       └── main.go
├── internal/                      # 内部包
│   ├── config/                    # 配置管理
│   │   └── config.go
│   ├── database/                 # 数据库连接
│   │   ├── postgres.go
│   │   └── migrations/
│   ├── models/                    # 数据模型
│   │   ├── market.go
│   │   ├── symbol.go
│   │   ├── kline.go
│   │   ├── box.go
│   │   ├── trend.go
│   │   ├── signal.go
│   │   ├── trade_track.go
│   │   ├── monitoring.go
│   │   └── user.go
│   ├── repository/               # 数据访问层
│   │   ├── market_repo.go
│   │   ├── symbol_repo.go
│   │   ├── kline_repo.go
│   │   ├── box_repo.go
│   │   ├── trend_repo.go
│   │   ├── signal_repo.go
│   │   ├── trade_track_repo.go
│   │   └── monitoring_repo.go
│   ├── service/                  # 业务逻辑层
│   │   ├── market/              # 行情服务
│   │   │   ├── fetcher_factory.go
│   │   │   ├── bybit_fetcher.go
│   │   │   ├── a_stock_fetcher.go
│   │   │   └── us_stock_fetcher.go
│   │   ├── strategy/            # 策略服务
│   │   │   ├── strategy_factory.go
│   │   │   ├── box_strategy.go
│   │   │   ├── trend_strategy.go
│   │   │   ├── key_level_strategy.go
│   │   │   └── volume_strategy.go
│   │   ├── monitoring/          # 实时监测服务
│   │   │   └── monitor_factory.go
│   │   ├── trading/            # 交易服务
│   │   │   ├── trade_executor.go
│   │   │   ├── stop_loss.go
│   │   │   └── position_sizer.go
│   │   ├── notification/       # 通知服务
│   │   │   └── feishu_notifier.go
│   │   └── ema/               # EMA计算服务
│   │       └── ema_calculator.go
│   ├── handler/                 # HTTP处理器
│   │   ├── auth_handler.go
│   │   ├── market_handler.go
│   │   ├── signal_handler.go
│   │   ├── trade_handler.go
│   │   ├── box_handler.go
│   │   ├── trend_handler.go
│   │   └── config_handler.go
│   ├── middleware/              # 中间件
│   │   ├── auth.go
│   │   ├── cors.go
│   │   └── logger.go
│   ├── websocket/               # WebSocket处理
│   │   ├── hub.go
│   │   ├── client.go
│   │   └── messages.go
│   └── router/                  # 路由
│       └── router.go
├── pkg/                         # 公共包
│   ├── response/               # 统一响应
│   │   └── response.go
│   ├── utils/                  # 工具函数
│   │   ├── time.go
│   │   ├── math.go
│   │   └── slice.go
│   └── validator/              # 参数验证
│       └── validator.go
├── config/                     # 配置文件
│   └── config.yml
├── db-scripts/                 # 数据库脚本
│   └── 001_init.sql
├── docs/                       # 文档
│   └── *.md
├── scripts/                    # 运维脚本
│   └── *.sh
├── Makefile                    # 构建文件
├── docker-compose.yml         # Docker部署
├── Dockerfile                 # 镜像构建
├── go.mod
└── go.sum
```

## 2. 前端项目结构 (Vue)

```
starfire-frontend/                # 前端项目根目录
├── public/                       # 静态资源
│   └── index.html
├── src/                         # 源代码
│   ├── api/                     # API接口
│   │   ├── index.js
│   │   ├── auth.js
│   │   ├── signals.js
│   │   ├── trades.js
│   │   ├── klines.js
│   │   └── ws.js
│   ├── assets/                  # 资源文件
│   │   ├── images/
│   │   └── styles/
│   │       ├── variables.scss   # 主题变量
│   │       ├── base.scss        # 基础样式
│   │       └── components.scss  # 组件样式
│   ├── components/              # 公共组件
│   │   ├── common/
│   │   │   ├── AppHeader.vue
│   │   │   ├── AppFooter.vue
│   │   │   ├── PageContainer.vue
│   │   │   └── Loading.vue
│   │   ├── charts/
│   │   │   ├── CandlestickChart.vue
│   │   │   ├── EquityCurve.vue
│   │   │   ├── WinRateChart.vue
│   │   │   ├── MonthlyChart.vue
│   │   │   └── PieChart.vue
│   │   ├── signals/
│   │   │   ├── SignalList.vue
│   │   │   ├── SignalCard.vue
│   │   │   └── SignalDetail.vue
│   │   └── trades/
│   │       ├── PositionList.vue
│   │       ├── TradeTable.vue
│   │       └── StatsCard.vue
│   ├── composables/            # 组合式函数
│   │   ├── useWebSocket.js
│   │   ├── useKline.js
│   │   └── useStats.js
│   ├── layouts/                # 布局
│   │   ├── DefaultLayout.vue
│   │   └── AuthLayout.vue
│   ├── router/                # 路由
│   │   └── index.js
│   ├── stores/                 # 状态管理
│   │   ├── auth.js
│   │   ├── signals.js
│   │   ├── trades.js
│   │   └── settings.js
│   ├── utils/                 # 工具函数
│   │   ├── formatters.js
│   │   ├── validators.js
│   │   └── chartConfig.js
│   ├── views/                 # 页面
│   │   ├── auth/
│   │   │   └── Login.vue
│   │   ├── dashboard/
│   │   │   └── Dashboard.vue
│   │   ├── signals/
│   │   │   ├── SignalList.vue
│   │   │   └── SignalDetail.vue
│   │   ├── trades/
│   │   │   ├── Positions.vue
│   │   │   ├── History.vue
│   │   │   └── Statistics.vue
│   │   ├── kline/
│   │   │   └── KlineChart.vue
│   │   └── settings/
│   │       └── Settings.vue
│   ├── App.vue
│   └── main.js
├── .env                       # 环境变量
├── .env.development
├── .env.production
├── package.json
├── vite.config.js
├── vue.config.js
└── README.md
```

## 3. Docker部署结构

```
┌─────────────────────────────────────────────────────────────────┐
│                        docker-compose.yml                        │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌─────────────┐                    ┌─────────────┐              │
│  │   starfire  │                    │  postgres   │              │
│  │   (Go API)  │                    │   (DB)      │              │
│  │             │                    │             │              │
│  │  Port:8080  │                    │  Port:5432  │              │
│  └──────┬──────┘                    └─────────────┘              │
│         │                                                        │
│         │ WebSocket                                              │
│         ▼                                                        │
│  ┌─────────────┐                                                │
│  │  starfire-  │                                                │
│  │   frontend  │                                                │
│  │   (Vue)     │                                                │
│  │  Port:3000  │                                                │
│  └─────────────┘                                                │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## 4. 核心模块交互关系

```
┌──────────────────────────────────────────────────────────────────────────┐
│                              HTTP/WebSocket                               │
│                         (API 请求 / 实时推送)                               │
└────────────────────────────────┬─────────────────────────────────────────┘
                                 │
                                 ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                          Handler 层 (HTTP)                                │
│  SignalHandler / TradeHandler / KlineHandler / ConfigHandler            │
└────────────────────────────────┬─────────────────────────────────────────┘
                                 │
                                 ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                           Service 层                                      │
│                                                                          │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌─────────────┐ │
│  │ MarketService│  │StrategySvc   │  │ TradingSvc   │  │NotifService│ │
│  │              │  │              │  │              │  │             │ │
│  │ - bybit      │  │ - box        │  │ - executor   │  │ - feishu    │ │
│  │ - a_stock    │  │ - trend      │  │ - stop_loss  │  │ - summary   │ │
│  │ - us_stock   │  │ - key_level  │  │ - position   │  │             │ │
│  └──────┬───────┘  │ - volume     │  │              │  └─────────────┘ │
│         │          └──────┬───────┘  └──────┬───────┘                   │
│         │                 │                 │                            │
│         ▼                 ▼                 ▼                            │
│  ┌────────────────────────────────────────────────────────────────────┐ │
│  │                    MonitoringService (实时监测)                       │ │
│  │                    - 订阅管理 - 事件触发 - 价格监测                   │ │
│  └────────────────────────────────┬───────────────────────────────────┘ │
└──────────────────────────────────┼──────────────────────────────────────┘
                                   │
                                   ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                         Repository 层 (数据访问)                          │
│  SignalRepo / TradeTrackRepo / KlineRepo / BoxRepo / TrendRepo           │
└────────────────────────────────┬──────────────────────────────────────────┘
                                   │
                                   ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                         PostgreSQL 数据库                                 │
│  signals / trade_tracks / klines / price_boxes / trends / key_levels     │
└──────────────────────────────────────────────────────────────────────────┘
```

## 5. Makefile 目标

```makefile
# 开发命令
make dev              # 本地开发启动
make dev-frontend     # 前端开发启动

# 构建命令
make build            # 编译后端
make build-frontend   # 编译前端
make docker-build     # 构建Docker镜像
make docker-build-amd64 # 构建AMD64架构镜像

# 部署命令
make docker-up        # Docker部署启动
make docker-down      # 停止Docker容器

# 数据库命令
make db-init          # 初始化数据库
make db-migrate       # 执行数据库迁移
make db-seed          # 填充测试数据

# 测试命令
make test             # 运行测试
make test-coverage    # 运行测试覆盖率

# 工具命令
make lint             # 代码检查
make fmt             # 代码格式化
```

## 6. 环境变量

```bash
# .env 文件示例
# 后端配置
APP_MODE=debug
APP_PORT=8080
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=your_password
DB_NAME=starfire_quant

# API密钥
BYBIT_API_KEY=your_api_key
BYBIT_API_SECRET=your_api_secret

# 飞书配置
FEISHU_WEBHOOK_URL=https://open.feishu.cn/open-apis/bot/v2/hook/xxx

# JWT配置
JWT_SECRET=your_jwt_secret
JWT_EXPIRES=24h
```
