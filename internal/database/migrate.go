package database

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/smallfire/starfire/internal/database/migrations"
	"github.com/smallfire/starfire/pkg/utils"
)

type migrationFile struct {
	Version     int64
	Description string
	Filename    string
}

// Migrator 负责执行数据库迁移
type Migrator struct {
	pool *pgxpool.Pool
}

// NewMigrator 创建一个新的迁移器
func NewMigrator(pool *pgxpool.Pool) *Migrator {
	return &Migrator{pool: pool}
}

// Run 执行所有待运行的迁移
func (m *Migrator) Run(ctx context.Context) error {
	if err := m.ensureMigrationsTable(ctx); err != nil {
		return fmt.Errorf("创建迁移版本表失败: %w", err)
	}

	files, err := m.parseMigrationFiles()
	if err != nil {
		return fmt.Errorf("解析迁移文件失败: %w", err)
	}

	// 兼容已有数据库：检测到表存在但无迁移记录时，标记当前版本为已应用
	if err := m.handleExistingDatabase(ctx, files); err != nil {
		return fmt.Errorf("处理已有数据库失败: %w", err)
	}

	applied, err := m.getAppliedVersions(ctx)
	if err != nil {
		return fmt.Errorf("查询已执行迁移失败: %w", err)
	}

	pending := 0
	for _, f := range files {
		if applied[f.Version] {
			continue
		}

		utils.Logger.Info("执行数据库迁移",
			zap.Int64("version", f.Version),
			zap.String("description", f.Description),
		)

		content, err := migrations.FS.ReadFile(f.Filename)
		if err != nil {
			return fmt.Errorf("读取迁移文件 %s 失败: %w", f.Filename, err)
		}

		tx, err := m.pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("开始迁移事务失败: %w", err)
		}

		_, err = tx.Exec(ctx, string(content))
		if err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("执行迁移 v%d (%s) 失败: %w", f.Version, f.Description, err)
		}

		_, err = tx.Exec(ctx,
			`INSERT INTO schema_migrations (version, description) VALUES ($1, $2)`,
			f.Version, f.Description,
		)
		if err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("记录迁移版本失败: %w", err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("提交迁移事务失败: %w", err)
		}

		utils.Logger.Info("迁移执行成功",
			zap.Int64("version", f.Version),
			zap.String("description", f.Description),
		)
		pending++
	}

	if pending == 0 {
		utils.Logger.Info("数据库迁移检查完成，无待执行的迁移")
	} else {
		utils.Logger.Info("数据库迁移完成", zap.Int("executed_count", pending))
	}

	return nil
}

func (m *Migrator) ensureMigrationsTable(ctx context.Context) error {
	_, err := m.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version     BIGINT PRIMARY KEY,
			description TEXT NOT NULL DEFAULT '',
			applied_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`)
	return err
}

func (m *Migrator) getAppliedVersions(ctx context.Context) (map[int64]bool, error) {
	rows, err := m.pool.Query(ctx, `SELECT version FROM schema_migrations ORDER BY version`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[int64]bool)
	for rows.Next() {
		var version int64
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = true
	}
	return applied, rows.Err()
}

func (m *Migrator) handleExistingDatabase(ctx context.Context, files []migrationFile) error {
	// 检查是否已有迁移记录
	var count int
	err := m.pool.QueryRow(ctx, `SELECT COUNT(*) FROM schema_migrations`).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil // 已有迁移记录，正常流程
	}

	// 检查是否已有业务表
	var exists bool
	err = m.pool.QueryRow(ctx,
		"SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_name = 'markets')",
	).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return nil // 全新数据库，正常执行迁移
	}

	// 已有数据库且无迁移记录，标记当前所有版本为已应用
	utils.Logger.Info("检测到已有数据库，标记现有迁移版本为已应用")
	for _, f := range files {
		_, err := m.pool.Exec(ctx,
			`INSERT INTO schema_migrations (version, description) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			f.Version, f.Description,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *Migrator) parseMigrationFiles() ([]migrationFile, error) {
	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		return nil, err
	}

	var files []migrationFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}

		f, err := parseMigrationFilename(name)
		if err != nil {
			utils.Logger.Warn("跳过无法解析的迁移文件", zap.String("file", name), zap.Error(err))
			continue
		}
		f.Filename = name
		files = append(files, f)
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Version < files[j].Version
	})

	return files, nil
}

// parseMigrationFilename 从文件名解析版本号和描述
// 格式: {6位序号}_{描述}.up.sql
func parseMigrationFilename(name string) (migrationFile, error) {
	base := strings.TrimSuffix(name, ".up.sql")
	parts := strings.SplitN(base, "_", 2)
	if len(parts) != 2 {
		return migrationFile{}, fmt.Errorf("文件名格式不正确: %s", name)
	}

	version, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return migrationFile{}, fmt.Errorf("版本号解析失败: %s", name)
	}

	return migrationFile{
		Version:     version,
		Description: parts[1],
	}, nil
}

// MigrationStatus 返回迁移状态信息（用于外部查询）
func (m *Migrator) MigrationStatus(ctx context.Context) ([]map[string]interface{}, error) {
	rows, err := m.pool.Query(ctx,
		`SELECT version, description, applied_at FROM schema_migrations ORDER BY version`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []map[string]interface{}
	for rows.Next() {
		var version int64
		var description string
		var appliedAt string
		if err := rows.Scan(&version, &description, &appliedAt); err != nil {
			return nil, err
		}
		result = append(result, map[string]interface{}{
			"version":     version,
			"description": description,
			"applied_at":  appliedAt,
		})
	}
	return result, rows.Err()
}
