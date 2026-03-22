# 需求文档：基础设施搭建

**需求编号**: REQ-INF-001
**模块**: 基础设施
**优先级**: P0
**状态**: 已完成 ✅
**完成时间**: 2024-03-22
**创建时间**: 2024-03-22

---

## 1. 需求概述

搭建星火量化系统的项目基础框架，包括：
- Go 后端项目结构
- Vue 前端项目结构
- 数据库初始化脚本
- 配置文件系统
- 日志系统
- Docker 部署配置

---

## 2. 后端项目结构

### 2.1 目录结构

按照以下结构创建 Go 项目：

```
starfire/                          # 后端项目根目录
├── cmd/                           # 应用程序入口
│   └── server/                    # 主服务入口
│       └── main.go
├── internal/                      # 内部包（禁止外部导入）
│   ├── config/                    # 配置管理
│   │   └── config.go
│   ├── database/                  # 数据库连接
│   │   └── postgres.go
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
│   ├── service/                   # 业务逻辑层
│   ├── handler/                   # HTTP处理器
│   ├── middleware/                # 中间件
│   ├── websocket/                 # WebSocket处理
│   └── router/                   # 路由
├── pkg/                          # 公共包（可外部导入）
│   ├── response/                  # 统一响应
│   │   └── response.go
│   └── utils/                     # 工具函数
├── config/                        # 配置文件
│   └── config.yml
├── db-scripts/                    # 数据库脚本（已存在）
├── docs/                          # 文档（已存在）
├── scripts/                       # 运维脚本
├── Makefile                       # 构建文件
├── docker-compose.yml            # Docker部署
├── Dockerfile                    # 镜像构建
├── go.mod
└── go.sum
```

### 2.2 Go 模块初始化

```bash
cd /Users/huangjicheng/go/src/github.com/smallfire
go mod init github.com/smallfire/starfire
```

### 2.3 依赖包

必须包含以下依赖：
- `github.com/gin-gonic/gin` - HTTP框架
- `github.com/golang-jwt/jwt/v5` - JWT认证
- `github.com/jackc/pgx/v5` - PostgreSQL驱动
- `github.com/spf13/viper` - 配置管理
- `github.com/gorilla/websocket` - WebSocket
- `go.uber.org/zap` - 日志
- `golang.org/x/crypto` - 密码加密

---

## 3. 前端项目结构

### 3.1 目录结构

使用 Vite + Vue 3 创建前端项目：

```
starfire-frontend/                 # 前端项目根目录
├── public/
├── src/
│   ├── api/                       # API接口
│   ├── assets/                    # 资源文件
│   │   └── styles/
│   │       ├── variables.scss    # 主题变量
│   │       └── global.scss
│   ├── components/                # 公共组件
│   ├── composables/               # 组合式函数
│   ├── layouts/                   # 布局
│   ├── router/                   # 路由
│   ├── stores/                   # 状态管理(Pinia)
│   ├── utils/                    # 工具函数
│   ├── views/                    # 页面
│   ├── App.vue
│   └── main.js
├── .env
├── package.json
├── vite.config.js
└── index.html
```

### 3.2 主题配置

**主色调**：绿色 `#00C853`
**背景色**：深色 `#0D1117`
**科技风格扁平设计**

```scss
// variables.scss
$primary: #00C853;
$primary-light: #69F0AE;
$primary-dark: #00C853;

$background: #0D1117;
$surface: #161B22;
$border: #30363D;

$text-primary: #E6EDF3;
$text-secondary: #8B949E;

$success: #26A69A;
$danger: #EF5350;
$warning: #FF9800;
```

---

## 4. 配置文件系统

### 4.1 config/config.yml

```yaml
# 系统基础配置
app:
  name: "星火量化"
  mode: "debug"
  host: "0.0.0.0"
  port: 8080
  timezone: "Asia/Shanghai"

# 数据库配置
database:
  host: "localhost"
  port: 5432
  user: "postgres"
  password: "${DB_PASSWORD}"
  dbname: "starfire_quant"
  sslmode: "disable"
  max_open_conns: 25
  max_idle_conns: 5

# 日志配置
log:
  level: "debug"
  format: "json"
  output: "stdout"

# 飞书通知配置
feishu:
  enabled: true
  webhook_url: "https://open.feishu.cn/open-apis/bot/v2/hook/c585be48-9114-4f71-bf3d-0f82f98ba4d6"
  send_summary: true
  summary_interval: 6

# JWT配置
jwt:
  secret: "${JWT_SECRET}"
  expires: "24h"

# 市场配置
markets:
  bybit:
    enabled: true
    symbols_limit: 200
    periods: ["15m", "1h"]
  a_stock:
    enabled: false
  us_stock:
    enabled: false

# 策略配置
strategies:
  box:
    enabled: true
    min_klines: 5
    width_threshold: 0.02
  trend:
    enabled: true
    ema_periods: [30, 60, 90]
  key_level:
    enabled: false
  volume_price:
    enabled: false

# 交易配置
trading:
  enabled: true
  initial_capital: 100000
  stop_loss_percent: 0.02
  take_profit_percent: 0.05
```

### 4.2 配置加载实现

使用 Viper 加载配置，支持环境变量覆盖：

```go
// internal/config/config.go
type Config struct {
    App      AppConfig
    Database DatabaseConfig
    Log      LogConfig
    Feishu   FeishuConfig
    JWT      JWTConfig
    Markets  MarketsConfig
    Strategies StrategiesConfig
    Trading  TradingConfig
}

func Load(configPath string) (*Config, error) {
    viper.SetConfigFile(configPath)
    viper.SetConfigType("yaml")
    viper.AutomaticEnv() // 支持环境变量

    // 环境变量覆盖
    viper.BindEnv("database.password", "DB_PASSWORD")
    viper.BindEnv("jwt.secret", "JWT_SECRET")

    if err := viper.ReadInConfig(); err != nil {
        return nil, err
    }

    var cfg Config
    if err := viper.Unmarshal(&cfg); err != nil {
        return nil, err
    }

    return &cfg, nil
}
```

---

## 5. 数据库连接

### 5.1 PostgreSQL 连接池

```go
// internal/database/postgres.go
type DB struct {
    *pgxpool.Pool
}

func NewPostgresDB(cfg DatabaseConfig) (*DB, error) {
    config, err := pgxpool.ParseConfig(cfg.DSN())
    if err != nil {
        return nil, err
    }

    config.MaxConns = cfg.MaxOpenConns
    config.MinConns = cfg.MaxIdleConns

    pool, err := pgxpool.NewWithConfig(context.Background(), config)
    if err != nil {
        return nil, err
    }

    // 测试连接
    if err := pool.Ping(context.Background()); err != nil {
        return nil, err
    }

    return &DB{Pool: pool}, nil
}
```

---

## 6. 日志系统

### 6.1 Zap 日志配置

```go
// pkg/utils/logger.go
var Logger *zap.Logger

func InitLogger(cfg LogConfig) {
    var level zapcore.Level
    zapcore.UnmarshalText([]byte(cfg.Level), &level)

    encoder := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
    if cfg.Format == "text" {
        encoder = zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())
    }

    writer := zapcore.AddSync(os.Stdout)
    if cfg.Output == "file" {
        file, _ := os.OpenFile(cfg.FilePath, os.O_APPEND|os.O_CREATE, 0644)
        writer = zapcore.AddSync(file)
    }

    core := zapcore.NewCore(encoder, writer, level)
    Logger = zap.New(core, zap.AddCaller(), zap.AddStacktrace(level))
}
```

---

## 7. 统一响应格式

```go
// pkg/response/response.go
type Response struct {
    Code    int         `json:"code"`
    Message string      `json:"message"`
    Data    interface{} `json:"data,omitempty"`
    Time    int64       `json:"timestamp"`
}

func Success(c *gin.Context, data interface{}) {
    c.JSON(200, Response{
        Code:    0,
        Message: "success",
        Data:    data,
        Time:    time.Now().Unix(),
    })
}

func Error(c *gin.Context, code int, message string) {
    c.JSON(200, Response{
        Code:    code,
        Message: message,
        Time:    time.Now().Unix(),
    })
}
```

---

## 8. Docker 配置

### 8.1 Dockerfile

```dockerfile
# 后端 Dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main ./cmd/server

FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /root/

COPY --from=builder /app/main .
COPY --from=builder /app/config ./config

EXPOSE 8080
CMD ["./main"]
```

### 8.2 docker-compose.yml

```yaml
version: '3.8'

services:
  postgres:
    image: postgres:18
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: ${DB_PASSWORD:-postgres}
      POSTGRES_DB: starfire_quant
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./db-scripts:/docker-entrypoint-initdb.d
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 10s
      timeout: 5s
      retries: 5

  starfire:
    build:
      context: .
      dockerfile: Dockerfile
    depends_on:
      postgres:
        condition: service_healthy
    ports:
      - "8080:8080"
    environment:
      DB_PASSWORD: ${DB_PASSWORD:-postgres}
      JWT_SECRET: ${JWT_SECRET:-your-secret-key}
    volumes:
      - ./config:/root/config:ro

volumes:
  postgres_data:
```

---

## 9. Makefile

### 9.1 必须实现的目标

```makefile
.PHONY: dev build docker-build docker-build-amd64 docker-up docker-down test lint fmt

# 开发
dev:
	@echo "Starting development server..."
	@cd cmd/server && go run main.go

# 构建
build:
	@echo "Building..."
	go build -o bin/starfire ./cmd/server

# Docker 构建 (AMD64)
docker-build-amd64:
	@echo "Building Docker image (amd64)..."
	docker build --platform linux/amd64 -t starfire:latest .

# Docker 部署
docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

# 测试
test:
	go test -v ./...

# 代码格式
fmt:
	go fmt ./...

# 代码检查
lint:
	golangci-lint run
```

---

## 10. 数据库初始化

**使用已存在的脚本**：`db-scripts/001_init.sql`

确保执行后创建以下表：
- markets
- symbols
- klines
- price_boxes
- trends
- key_levels
- signals
- trade_tracks
- monitorings
- users
- configs
- notifications

以及默认数据：
- 3个市场（bybit, a_stock, us_stock）
- 1个管理员用户（admin/admin123）

---

## 11. 验收标准

### 11.1 后端验收

- [ ] `go mod init` 成功
- [ ] `go build` 编译通过
- [ ] 配置文件加载正常
- [ ] 数据库连接成功
- [ ] 日志系统正常输出
- [ ] `make dev` 能启动服务

### 11.2 前端验收

- [ ] `npm create vite@latest` 创建成功
- [ ] 主题样式配置正确
- [ ] `npm run dev` 能启动前端
- [ ] 依赖安装正常

### 11.3 Docker验收

- [ ] `make docker-build-amd64` 构建成功
- [ ] `make docker-up` 启动成功
- [ ] 服务能正常访问

### 11.4 数据库验收

- [ ] 所有表创建成功
- [ ] 默认数据插入成功
- [ ] 自动更新 `updated_at` 触发器生效

---

## 12. 注意事项

1. 所有配置必须支持环境变量覆盖
2. 使用 UTC+8 时区处理时间
3. 日志输出 JSON 格式（便于收集分析）
4. 数据库密码不能硬编码
5. 代码遵循 Go 标准项目布局

---

**执行人**: 待分配
**预计工时**: 4小时
**实际完成时间**: 待填写
