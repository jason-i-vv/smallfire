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

# Docker 构建
docker-build:
	@echo "Building Docker image..."
	docker build -t starfire:latest .

# Docker 部署
docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

# 数据库初始化
db-init:
	@echo "Initializing database..."
	docker-compose exec postgres psql -U postgres -d starfire_quant -f /docker-entrypoint-initdb.d/001_init.sql

# 测试
test:
	go test -v ./...

# 代码格式
fmt:
	go fmt ./...

# 代码检查
lint:
	golangci-lint run

# 依赖更新
deps:
	go mod tidy

# 查看依赖
deps-list:
	go list -m all
