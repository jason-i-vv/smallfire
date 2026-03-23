.PHONY: dev backend frontend docker-dev docker-build docker-build-amd64 docker-up docker-down test lint fmt deps

# ============================================
# 开发模式
# ============================================

# 启动后端服务（需要先启动PostgreSQL）
backend:
	@echo "Starting backend server..."
	@go run ./cmd/server/main.go

# 启动前端服务（需要在starfire-frontend目录）
frontend:
	@echo "Starting frontend server..."
	@cd starfire-frontend && npm run dev

# 同时启动前端和后端（需要分别打开两个终端）
# 终端1: make backend
# 终端2: make frontend
dev: backend

# 启动本地开发所需的数据库
db-start:
	@echo "Starting PostgreSQL database..."
	@docker-compose up -d postgres

# 停止数据库
db-stop:
	@echo "Stopping PostgreSQL database..."
	@docker-compose stop postgres

# 数据库连接
db-connect:
	@docker exec -it smallfire-postgres-1 psql -U postgres -d starfire_quant

# 数据库初始化
db-init:
	@echo "Initializing database..."
	@docker-compose exec postgres psql -U postgres -d starfire_quant -f /docker-entrypoint-initdb.d/001_init.sql

# 数据库重置
db-reset:
	@echo "Resetting database..."
	@docker-compose down -v
	@docker-compose up -d postgres
	@sleep 5
	@make db-init

# ============================================
# Docker 开发模式（推荐）
# ============================================

# 启动所有服务（后端 + 前端 + 数据库）通过Docker
docker-dev:
	@echo "Starting all services with Docker..."
	@docker-compose -f docker-compose.dev.yml up -d

# 停止所有Docker服务
docker-dev-down:
	@echo "Stopping all Docker services..."
	@docker-compose -f docker-compose.dev.yml down

# 查看Docker服务日志
docker-dev-logs:
	@docker-compose -f docker-compose.dev.yml logs -f

# ============================================
# Docker 生产构建
# ============================================

# 构建后端镜像 (AMD64)
docker-build-amd64:
	@echo "Building backend Docker image (amd64)..."
	@docker build --platform linux/amd64 -t starfire:latest .

# 构建后端镜像
docker-build:
	@echo "Building backend Docker image..."
	@docker build -t starfire:latest .

# Docker 部署（仅后端+数据库）
docker-up:
	@docker-compose up -d

docker-down:
	@docker-compose down

# ============================================
# 前端相关
# ============================================

# 安装前端依赖
frontend-install:
	@cd starfire-frontend && npm install

# 前端构建
frontend-build:
	@cd starfire-frontend && npm run build

# ============================================
# 测试和代码质量
# ============================================

test:
	go test -v ./...

lint:
	golangci-lint run

fmt:
	go fmt ./...

# ============================================
# 依赖管理
# ============================================

deps:
	go mod tidy

deps-list:
	go list -m all

# ============================================
# 帮助
# ============================================

help:
	@echo "星火量化 - Makefile"
	@echo ""
	@echo "本地开发模式:"
	@echo "  make db-start       - 启动PostgreSQL数据库"
	@echo "  make db-stop        - 停止PostgreSQL数据库"
	@echo "  make db-reset      - 重置数据库"
	@echo "  make backend        - 启动后端服务"
	@echo "  make frontend       - 启动前端服务"
	@echo ""
	@echo "Docker开发模式（推荐）:"
	@echo "  make docker-dev     - 启动所有服务(后端+前端+数据库)"
	@echo "  make docker-dev-down - 停止所有Docker服务"
	@echo "  make docker-dev-logs - 查看Docker服务日志"
	@echo ""
	@echo "Docker生产构建:"
	@echo "  make docker-build   - 构建后端镜像"
	@echo "  make docker-up      - 启动生产服务"
	@echo "  make docker-down    - 停止生产服务"
	@echo ""
	@echo "前端:"
	@echo "  make frontend-install - 安装前端依赖"
	@echo "  make frontend-build   - 构建前端"
	@echo ""
	@echo "其他:"
	@echo "  make test           - 运行测试"
	@echo "  make lint           - 代码检查"
	@echo "  make fmt            - 代码格式化"
