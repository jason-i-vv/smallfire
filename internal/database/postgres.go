package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/smallfire/starfire/internal/config"
)

type DB struct {
	*pgxpool.Pool
}

func NewPostgresDB(cfg config.DatabaseConfig) (*DB, error) {
	connStr := cfg.DSN()

	poolConfig, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("解析数据库配置失败: %w", err)
	}

	poolConfig.MaxConns = int32(cfg.MaxOpenConns)
	poolConfig.MinConns = int32(cfg.MaxIdleConns)

	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return nil, fmt.Errorf("创建数据库连接池失败: %w", err)
	}

	// 数据库使用 UTC 时区，不做任何时区转换
	if _, err := pool.Exec(context.Background(), "SET TimeZone = 'UTC'"); err != nil {
		pool.Close()
		return nil, fmt.Errorf("设置时区失败: %w", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("数据库连接测试失败: %w", err)
	}

	// 执行数据库自动迁移
	migrator := NewMigrator(pool)
	if err := migrator.Run(context.Background()); err != nil {
		pool.Close()
		return nil, fmt.Errorf("数据库迁移失败: %w", err)
	}

	return &DB{Pool: pool}, nil
}

func (db *DB) Close() {
	if db.Pool != nil {
		db.Pool.Close()
	}
}
