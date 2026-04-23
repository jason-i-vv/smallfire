# 星火量化 - 生产部署文档

## 架构概览

```
客户端 → Nginx (前端 :80) → 后端 API (:8080) → PostgreSQL (:5432)
```

| 组件 | 镜像 | 端口 |
|------|------|------|
| 前端 | `starfire-frontend` (Nginx) | 80 |
| 后端 | `starfire-backend` (Go) | 8080 |
| 数据库 | `postgres:18` | 5432 |

## 前置条件

- Docker 20.10+ / Docker Compose v2
- 服务器架构：**x86_64 (AMD64)**
- 阿里云 ACR 仓库登录凭证

## 一、构建并推送镜像

### 1. 登录阿里云容器镜像服务

```bash
docker login registry.cn-hangzhou.aliyuncs.com
```

### 2. 构建并推送（一键完成）

```bash
make docker-push
```

该命令会自动：
1. 构建后端镜像 `starfire-backend:latest` (amd64)
2. 构建前端镜像 `starfire-frontend:latest` (amd64)
3. 打标签到 `registry.cn-hangzhou.aliyuncs.com/deepcoin/`
4. 推送到远程仓库

### 3. 分步操作（可选）

```bash
# 只构建，不推送
make docker-build-all

# 推送指定版本号
make docker-push IMAGE_TAG=v1.0.0
```

最终镜像地址：
- `registry.cn-hangzhou.aliyuncs.com/deepcoin/starfire-backend:latest`
- `registry.cn-hangzhou.aliyuncs.com/deepcoin/starfire-frontend:latest`

## 二、服务器部署

### 1. 准备目录结构

```bash
mkdir -p /opt/starfire && cd /opt/starfire
```

### 2. 创建配置文件

将 `config/config.yml` 放到 `/opt/starfire/config/config.yml`，并修改生产环境配置：

```yaml
app:
  mode: "release"        # 改为 release
  host: "0.0.0.0"
  port: 8080

database:
  host: "postgres"       # Docker 内部服务名
  port: 5432
  user: "postgres"
  password: "你的强密码"  # 生产环境务必修改
  dbname: "starfire_quant"
  sslmode: "disable"

log:
  level: "info"          # 生产环境用 info
  format: "json"
  output: "stdout"

jwt:
  enabled: true
  secret: "你的JWT密钥"  # 生产环境务必修改
  expires: "24h"
```

### 3. 创建环境变量文件

```bash
cat > /opt/starfire/.env << 'EOF'
DB_PASSWORD=你的数据库密码
JWT_SECRET=你的JWT密钥
AI_API_KEY=你的AI接口Key
REGISTRY=registry.cn-hangzhou.aliyuncs.com/deepcoin
IMAGE_TAG=latest
EOF
chmod 600 /opt/starfire/.env
```

### 4. 上传 docker-compose.yml

将 `docker-compose.yml` 上传到 `/opt/starfire/docker-compose.yml`。

### 5. 拉取镜像并启动

```bash
cd /opt/starfire

# 拉取最新镜像
docker compose pull

# 启动所有服务
docker compose up -d

# 查看状态
docker compose ps

# 查看日志
docker compose logs -f
```

### 6. 验证部署

```bash
# 检查前端
curl -I http://localhost

# 检查后端 API
curl http://localhost:8080/api/v1/health || true

# 检查数据库连接
docker exec starfire-postgres pg_isready -U postgres
```

## 三、运维操作

### 更新部署

```bash
cd /opt/starfire

# 拉取最新镜像
docker compose pull

# 重新创建容器（零停机可加 --no-down-time 策略）
docker compose up -d
```

### 回滚到指定版本

```bash
# 修改 .env 中的 IMAGE_TAG
sed -i 's/IMAGE_TAG=.*/IMAGE_TAG=v1.0.0/' .env

docker compose up -d
```

### 查看日志

```bash
# 所有服务日志
docker compose logs -f --tail=200

# 单个服务日志
docker compose logs -f backend
docker compose logs -f frontend
docker compose logs -f postgres
```

### 数据库备份

```bash
# 手动备份
docker exec starfire-postgres pg_dump -U postgres starfire_quant > backup_$(date +%Y%m%d_%H%M%S).sql

# 恢复备份
cat backup_20260423.sql | docker exec -i starfire-postgres psql -U postgres starfire_quant
```

### 停止服务

```bash
docker compose down        # 停止容器，保留数据
docker compose down -v     # 停止容器并删除数据卷（危险！）
```

## 四、注意事项

1. **安全**：生产环境必须修改 `DB_PASSWORD`、`JWT_SECRET`，不要使用默认值
2. **数据持久化**：PostgreSQL 数据存储在 Docker 命名卷 `postgres_data` 中，`docker compose down` 不会删除数据，但 `docker compose down -v` 会
3. **防火墙**：只开放 80 端口（前端），5432 和 8080 不需要对外暴露
4. **HTTPS**：生产环境建议在前面再加一层 Nginx/Caddy 做反向代理和 TLS 终结
5. **监控**：建议配置 `docker compose logs` 的日志轮转，避免磁盘写满
