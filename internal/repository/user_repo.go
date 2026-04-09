package repository

import (
	"context"
	"fmt"

	"github.com/smallfire/starfire/internal/database"
	"github.com/smallfire/starfire/internal/models"
)

// UserRepoPG 用户数据访问实现
type UserRepoPG struct {
	db *database.DB
}

// NewUserRepoPG 创建用户数据访问实例
func NewUserRepoPG(db *database.DB) UserRepo {
	return &UserRepoPG{
		db: db,
	}
}

func (r *UserRepoPG) GetByUsername(username string) (*models.User, error) {
	var user models.User
	query := `SELECT id, username, password_hash, COALESCE(nickname, ''), role, is_active, last_login_at, created_at, updated_at
		FROM users WHERE username = $1`

	err := r.db.QueryRow(context.Background(), query, username).Scan(
		&user.ID, &user.Username, &user.PasswordHash, &user.Nickname,
		&user.Role, &user.IsActive, &user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}

	return &user, nil
}

func (r *UserRepoPG) GetByID(id int) (*models.User, error) {
	var user models.User
	query := `SELECT id, username, password_hash, COALESCE(nickname, ''), role, is_active, last_login_at, created_at, updated_at
		FROM users WHERE id = $1`

	err := r.db.QueryRow(context.Background(), query, id).Scan(
		&user.ID, &user.Username, &user.PasswordHash, &user.Nickname,
		&user.Role, &user.IsActive, &user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}

	return &user, nil
}

func (r *UserRepoPG) Create(user *models.User) error {
	query := `INSERT INTO users (username, password_hash, nickname, role, is_active)
		VALUES ($1, $2, $3, $4, $5) RETURNING id, created_at, updated_at`

	err := r.db.QueryRow(context.Background(), query,
		user.Username, user.PasswordHash, user.Nickname, user.Role, user.IsActive,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return fmt.Errorf("创建用户失败: %w", err)
	}

	return nil
}

func (r *UserRepoPG) UpdatePassword(id int, passwordHash string) error {
	query := `UPDATE users SET password_hash = $1 WHERE id = $2`
	_, err := r.db.Exec(context.Background(), query, passwordHash, id)
	if err != nil {
		return fmt.Errorf("更新密码失败: %w", err)
	}
	return nil
}

func (r *UserRepoPG) UpdateLastLoginAt(id int) error {
	query := `UPDATE users SET last_login_at = CURRENT_TIMESTAMP WHERE id = $1`
	_, err := r.db.Exec(context.Background(), query, id)
	if err != nil {
		return fmt.Errorf("更新登录时间失败: %w", err)
	}
	return nil
}

func (r *UserRepoPG) UpdateIsActive(id int, isActive bool) error {
	query := `UPDATE users SET is_active = $1 WHERE id = $2`
	_, err := r.db.Exec(context.Background(), query, isActive, id)
	if err != nil {
		return fmt.Errorf("更新用户状态失败: %w", err)
	}
	return nil
}

func (r *UserRepoPG) List() ([]*models.User, error) {
	query := `SELECT id, username, password_hash, COALESCE(nickname, ''), role, is_active, last_login_at, created_at, updated_at
		FROM users ORDER BY created_at DESC`

	rows, err := r.db.Query(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("查询用户列表失败: %w", err)
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		var user models.User
		err := rows.Scan(
			&user.ID, &user.Username, &user.PasswordHash, &user.Nickname,
			&user.Role, &user.IsActive, &user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("扫描用户数据失败: %w", err)
		}
		users = append(users, &user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历用户列表失败: %w", err)
	}

	return users, nil
}

func (r *UserRepoPG) ExistsByUsername(username string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)`
	err := r.db.QueryRow(context.Background(), query, username).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("检查用户名是否存在失败: %w", err)
	}
	return exists, nil
}
