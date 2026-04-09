package auth

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/internal/repository"
	"github.com/smallfire/starfire/internal/utils"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// AuthService 认证服务
type AuthService struct {
	userRepo  repository.UserRepo
	jwtSecret string
	jwtExpire time.Duration
	logger    *zap.Logger
}

// NewAuthService 创建认证服务
func NewAuthService(userRepo repository.UserRepo, jwtSecret string, jwtExpire time.Duration, logger *zap.Logger) *AuthService {
	return &AuthService{
		userRepo:  userRepo,
		jwtSecret: jwtSecret,
		jwtExpire: jwtExpire,
		logger:    logger,
	}
}

// AuthResult 认证结果
type AuthResult struct {
	User  *models.User `json:"user"`
	Token string       `json:"token"`
}

var (
	// ErrDuplicateUsername 用户名已存在
	ErrDuplicateUsername = errors.New("用户名已存在")
	// ErrInvalidCredentials 用户名或密码错误
	ErrInvalidCredentials = errors.New("用户名或密码错误")
	// ErrUserDisabled 用户已被禁用
	ErrUserDisabled = errors.New("用户已被禁用")
	// ErrOldPasswordMismatch 旧密码不匹配
	ErrOldPasswordMismatch = errors.New("旧密码错误")
	// ErrValidation 参数验证失败
	ErrValidation = errors.New("参数验证失败")

	usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_]{3,32}$`)
)

// Register 用户注册
func (s *AuthService) Register(username, password, nickname string) (*AuthResult, error) {
	if err := s.validateRegisterParams(username, password, nickname); err != nil {
		return nil, err
	}

	exists, err := s.userRepo.ExistsByUsername(username)
	if err != nil {
		return nil, fmt.Errorf("检查用户名失败: %w", err)
	}
	if exists {
		return nil, ErrDuplicateUsername
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("密码哈希失败: %w", err)
	}

	user := &models.User{
		Username:     username,
		PasswordHash: string(passwordHash),
		Nickname:     nickname,
		Role:         models.RoleUser,
		IsActive:     true,
	}

	if err := s.userRepo.Create(user); err != nil {
		return nil, fmt.Errorf("创建用户失败: %w", err)
	}

	token, err := utils.GenerateToken(user.ID, user.Username, user.Role, s.jwtSecret, s.jwtExpire)
	if err != nil {
		return nil, fmt.Errorf("生成token失败: %w", err)
	}

	s.logger.Info("新用户注册",
		zap.String("username", username),
		zap.Int("user_id", user.ID),
	)

	return &AuthResult{User: user, Token: token}, nil
}

// Login 用户登录
func (s *AuthService) Login(username, password string) (*AuthResult, error) {
	if username == "" || password == "" {
		return nil, ErrInvalidCredentials
	}

	user, err := s.userRepo.GetByUsername(username)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	if !user.IsActive {
		return nil, ErrUserDisabled
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	// 更新登录时间
	if err := s.userRepo.UpdateLastLoginAt(user.ID); err != nil {
		s.logger.Warn("更新登录时间失败", zap.Error(err))
	}
	user.LastLoginAt = nil // 重新查询来获取最新时间（不关键，跳过）

	token, err := utils.GenerateToken(user.ID, user.Username, user.Role, s.jwtSecret, s.jwtExpire)
	if err != nil {
		return nil, fmt.Errorf("生成token失败: %w", err)
	}

	s.logger.Info("用户登录",
		zap.String("username", username),
		zap.Int("user_id", user.ID),
	)

	return &AuthResult{User: user, Token: token}, nil
}

// ChangePassword 修改密码
func (s *AuthService) ChangePassword(userID int, oldPassword, newPassword string) error {
	if newPassword == "" {
		return ErrValidation
	}

	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return fmt.Errorf("查询用户失败: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)); err != nil {
		return ErrOldPasswordMismatch
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("密码哈希失败: %w", err)
	}

	if err := s.userRepo.UpdatePassword(userID, string(passwordHash)); err != nil {
		return fmt.Errorf("更新密码失败: %w", err)
	}

	s.logger.Info("用户修改密码", zap.Int("user_id", userID))
	return nil
}

// GetUserByID 根据ID获取用户
func (s *AuthService) GetUserByID(id int) (*models.User, error) {
	user, err := s.userRepo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}
	return user, nil
}

// ListUsers 获取所有用户
func (s *AuthService) ListUsers() ([]*models.User, error) {
	users, err := s.userRepo.List()
	if err != nil {
		return nil, fmt.Errorf("查询用户列表失败: %w", err)
	}
	return users, nil
}

// UpdateUserStatus 更新用户状态
func (s *AuthService) UpdateUserStatus(id int, isActive bool) error {
	if err := s.userRepo.UpdateIsActive(id, isActive); err != nil {
		return fmt.Errorf("更新用户状态失败: %w", err)
	}

	s.logger.Info("更新用户状态",
		zap.Int("user_id", id),
		zap.Bool("is_active", isActive),
	)
	return nil
}

// ResetPassword 重置用户密码（管理员）
func (s *AuthService) ResetPassword(id int, newPassword string) error {
	if newPassword == "" {
		return ErrValidation
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("密码哈希失败: %w", err)
	}

	if err := s.userRepo.UpdatePassword(id, string(passwordHash)); err != nil {
		return fmt.Errorf("重置密码失败: %w", err)
	}

	s.logger.Info("管理员重置密码", zap.Int("user_id", id))
	return nil
}

func (s *AuthService) validateRegisterParams(username, password, nickname string) error {
	if !usernameRegex.MatchString(username) {
		return fmt.Errorf("用户名必须为3-32位的字母、数字或下划线: %w", ErrValidation)
	}
	if len(password) < 6 || len(password) > 64 {
		return fmt.Errorf("密码长度必须在6-64位之间: %w", ErrValidation)
	}
	if len(nickname) > 32 {
		return fmt.Errorf("昵称长度不能超过32位: %w", ErrValidation)
	}
	return nil
}
