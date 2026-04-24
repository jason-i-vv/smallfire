---
name: deploy-to-server
description: |
  构建 Docker 镜像，推送到阿里云 ACR，然后部署到远程服务器。
  执行流程：构建后端镜像 → 推送镜像 → SSH 到服务器拉取镜像 → 重启服务 → 检查状态。
triggers:
  - /deploy-to-server
  - deploy to server
allowed-tools:
  - Bash
  - Read
---

## 部署到服务器

### 步骤 1: 构建并推送镜像

```bash
# 构建后端和前端镜像 (AMD64)
make docker-build-all

# 推送到阿里云 ACR
docker push registry.cn-hangzhou.aliyuncs.com/deepcoin/starfire-backend:latest
docker push registry.cn-hangzhou.aliyuncs.com/deepcoin/starfire-frontend:latest
```

### 步骤 2: SSH 到服务器执行更新

```bash
ssh ubuntu@150.109.233.168 << 'EOF'
cd /home/ubuntu/starfire

# 拉取最新镜像
sudo docker-compose pull

# 重启服务
sudo docker-compose up -d --remove-orphans

# 检查服务状态
sudo docker-compose ps

# 检查日志
sudo docker-compose logs --tail=20
EOF
```

### 步骤 3: 验证部署

```bash
# 检查远程服务健康状态
ssh ubuntu@150.109.233.168 "curl -s http://localhost:8080/health || echo 'Health check failed'"
```
