# 星火量化 - 微型量化交易系统

星星之火，点燃你的交易人生。一个辅助交易的微型量化交易系统。

## 项目架构

### 技术栈
- **后端**：Golang 1.21
- **前端**：Vue 3 + Vite + Pinia + Vue Router
- **数据库**：PostgreSQL 18.0
- **配置**：YAML 格式
- **日志**：Zap (JSON格式)
- **部署**：Docker + Docker Compose

### 功能模块
- 行情抓取（Bybit/A股/美股）
- 策略分析（箱体突破/趋势/阻力支撑/量价异常）
- 实时监测
- 交易跟踪
- 飞书通知
- 前端控制台

## 快速开始

### 1. 环境准备
- Go 1.21+
- Node.js 18+
- PostgreSQL 18
- Docker (可选)

### 2. 安装依赖

#### 后端
```bash
cd /Users/huangjicheng/go/src/github.com/smallfire
go mod tidy
```

#### 前端
```bash
cd starfire-frontend
npm install
```

### 3. 配置文件

复制示例配置：
```bash
cp .env.example .env
```

编辑配置文件 `config/config.yml`。

### 4. 初始化数据库

启动 PostgreSQL 并执行初始化脚本：
```bash
psql -U postgres -d starfire_quant -f db-scripts/001_init.sql
```

### 5. 运行项目

#### 开发模式
```bash
# 启动后端
make dev

# 启动前端（新终端）
cd starfire-frontend
npm run dev
```

#### Docker 部署
```bash
# 构建镜像
make docker-build-amd64

# 启动服务
make docker-up

# 查看日志
docker-compose logs -f

# 停止服务
make docker-down
```

### 6. 访问项目

- 前端地址：http://localhost:3000
- 后端API：http://localhost:8080
- 健康检查：http://localhost:8080/health

## 项目结构

### 后端
```
starfire/
├── cmd/server/              # 主程序入口
├── internal/
│   ├── config/             # 配置管理
│   ├── database/          # 数据库连接
│   ├── models/            # 数据模型
│   ├── router/            # 路由
│   ├── service/           # 业务逻辑（预留）
│   ├── handler/           # HTTP处理器（预留）
│   ├── repository/        # 数据访问（预留）
│   └── middleware/        # 中间件（预留）
├── pkg/
│   ├── response/          # 统一响应
│   └── utils/             # 工具函数
├── config/                # 配置文件
├── db-scripts/            # 数据库脚本
├── docs/                  # 文档
└── scripts/               # 运维脚本
```

### 前端
```
starfire-frontend/
├── src/
│   ├── api/               # API接口
│   ├── assets/            # 静态资源
│   ├── components/        # 公共组件
│   ├── layouts/           # 布局
│   ├── views/             # 页面
│   ├── router/            # 路由
│   ├── stores/            # 状态管理
│   └── utils/             # 工具函数
├── package.json
└── vite.config.js
```

## 开发命令

```bash
make dev                # 启动开发服务器
make build              # 编译后端
make docker-build-amd64 # 构建Docker镜像（AMD64）
make docker-up          # Docker启动
make docker-down        # Docker停止
make db-init            # 初始化数据库
make test               # 运行测试
make fmt                # 格式化代码
```

## 需求文档

所有需求文档位于 `pdms/` 目录。

---

**文档版本**: v1.0
**创建时间**: 2024-03-22
**项目地址**: https://github.com/smallfire/starfire
