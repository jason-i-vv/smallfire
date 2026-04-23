REGISTRY ?= registry.cn-hangzhou.aliyuncs.com/deepcoin
IMAGE_TAG ?= latest

.PHONY: dev backend frontend restart docker-dev docker-build docker-build-amd64 docker-push docker-deploy docker-up docker-down test lint fmt deps

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

# 重启后端服务（先杀掉旧进程，再启动新的）
restart:
	@echo "Stopping old backend server..."
	@pkill -f 'go run ./cmd/server/main.go' 2>/dev/null || true
	@pkill -f 'go-build.*cmd/server/main.go' 2>/dev/null || true
	@pkill -f '/tmp/go-build.*/main' 2>/dev/null || true
	@lsof -ti :8080 | xargs kill -9 2>/dev/null || true
	@sleep 1
	@echo "Starting backend server..."
	@go run ./cmd/server/main.go

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

# 数据库初始化（迁移在程序启动时自动执行）
db-init:
	@echo "数据库迁移在程序启动时自动执行，无需手动初始化"

# 数据库重置
db-reset:
	@echo "Resetting database..."
	@docker-compose down -v
	@docker-compose up -d postgres
	@sleep 5
	@echo "数据库重置完成，下次启动程序时将自动执行迁移"

# 查看迁移状态
db-migrate-status:
	@echo "查看迁移状态..."
	@docker exec -it starfire-postgres psql -U postgres -d starfire_quant \
		-c "SELECT version, description, applied_at FROM schema_migrations ORDER BY version;"

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
	@docker build --platform linux/amd64 \
		-t starfire-backend:latest \
		-t $(REGISTRY)/starfire-backend:$(IMAGE_TAG) .

# 构建前端镜像 (AMD64)
docker-build-frontend:
	@echo "Building frontend Docker image (amd64)..."
	@docker build --platform linux/amd64 -f Dockerfile.frontend \
		-t starfire-frontend:latest \
		-t $(REGISTRY)/starfire-frontend:$(IMAGE_TAG) .

# 构建所有镜像 (AMD64)
docker-build-all: docker-build-amd64 docker-build-frontend

# 构建后端镜像
docker-build:
	@echo "Building backend Docker image..."
	@docker build \
		-t starfire-backend:latest \
		-t $(REGISTRY)/starfire-backend:$(IMAGE_TAG) .

# 推送到阿里云 ACR
docker-push: docker-build-all
	@echo "Pushing images to $(REGISTRY) ..."
	@docker push $(REGISTRY)/starfire-backend:$(IMAGE_TAG)
	@docker push $(REGISTRY)/starfire-frontend:$(IMAGE_TAG)
	@echo "Done! Images pushed:"
	@echo "  $(REGISTRY)/starfire-backend:$(IMAGE_TAG)"
	@echo "  $(REGISTRY)/starfire-frontend:$(IMAGE_TAG)"

# Docker 部署（后端+数据库+前端）
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

# 清理交易数据（开平仓逻辑变更后使用）
db-cleanup:
	@bash scripts/cleanup_trading_data.sh "手动执行清理"

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
	@echo "  make restart        - 重启后端服务（杀旧进程+启动新进程）"
	@echo "  make frontend       - 启动前端服务"
	@echo ""
	@echo "Docker开发模式（推荐）:"
	@echo "  make docker-dev     - 启动所有服务(后端+前端+数据库)"
	@echo "  make docker-dev-down - 停止所有Docker服务"
	@echo "  make docker-dev-logs - 查看Docker服务日志"
	@echo ""
	@echo "Docker生产构建:"
	@echo "  make docker-build-amd64    - 构建后端镜像 (amd64)"
	@echo "  make docker-build-frontend - 构建前端镜像 (amd64)"
	@echo "  make docker-build-all      - 构建所有镜像"
	@echo "  make docker-push           - 构建+推送镜像到 ACR"
	@echo "  make docker-up             - 启动生产服务"
	@echo "  make docker-down           - 停止生产服务"
	@echo ""
	@echo "前端:"
	@echo "  make frontend-install - 安装前端依赖"
	@echo "  make frontend-build   - 构建前端"
	@echo ""
	@echo "数据清理:"
	@echo "  make db-cleanup     - 清理交易数据（交互式确认）"
	@echo ""
	@echo "其他:"
	@echo "  make test           - 运行测试"
	@echo "  make lint           - 代码检查"
	@echo "  make fmt            - 代码格式化"
