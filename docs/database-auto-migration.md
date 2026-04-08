# 数据库自动迁移

## 变更日期
2026-04-06

## 背景

之前数据库初始化依赖 docker-compose 的 `/docker-entrypoint-initdb.d/` 机制，只在数据库首次创建时执行 `db-scripts/` 下的 SQL 脚本。存在以下问题：
1. 重建数据库时 init 脚本可能报错（如引用不存在的触发器）
2. 无法追踪已执行哪些脚本
3. 每次新增表/字段都需要手动管理 init 脚本顺序

## 改造方案

自研轻量迁移系统，使用已有的 `pgx/v5` + Go 标准库 `embed.FS`，零新增依赖。

### 核心机制

- 程序启动时自动检查并执行未运行的迁移
- 使用 `schema_migrations` 表追踪已执行的版本
- 迁移 SQL 文件通过 `embed.FS` 编译进二进制
- 每个迁移在一个事务中执行（全部成功或全部回滚）

### 迁移文件位置

```
internal/database/migrations/
  embed.go                              # embed.FS 声明
  000001_init_schema.up.sql             # 初始 schema（12 张表 + 索引 + 触发器）
  000002_supplement_indexes.up.sql      # 补充索引
  000003_add_monitoring_symbol_code.up.sql  # monitorings 添加 symbol_code
  000004_add_period_to_price_boxes.up.sql   # price_boxes 添加 period
```

### 新增迁移

在 `internal/database/migrations/` 下添加新文件，命名格式：`{6位序号}_{描述}.up.sql`

例如：`000005_add_new_table.up.sql`

程序启动时会自动检测并执行新迁移。

### 兼容性

| 场景 | 行为 |
|------|------|
| 全新数据库 | 按顺序执行所有迁移 |
| 已有数据库（旧 init 初始化过） | 自动检测，标记当前版本为已应用，不重复执行 SQL |
| 后续新增迁移 | 只执行未应用的迁移版本 |

### 相关文件变更

| 文件 | 变更 |
|------|------|
| `internal/database/migrate.go` | 新建，迁移核心逻辑 |
| `internal/database/migrations/` | 新建目录，存放迁移 SQL 文件 |
| `internal/database/postgres.go` | 修改，NewPostgresDB 中集成迁移调用 |
| `docker-compose.yml` | 移除 db-scripts volume 挂载 |
| `docker-compose.dev.yml` | 移除 db-scripts volume 挂载 |
| `Makefile` | 更新 db-reset，新增 db-migrate-status |

### 旧脚本归档

一次性数据修复脚本已移至 `db-scripts/archive/`：
- fix_kline_close_time.sql
- fix_kline_close_time_simple.sql
- fix_kline_timezone_and_backfill.sql
- remove_duplicate_boxes.sql

### 运维命令

```bash
# 重置数据库（删除数据重建）
make db-reset

# 重启后端（迁移在启动时自动执行）
make backend

# 查看已执行的迁移版本
make db-migrate-status
```
