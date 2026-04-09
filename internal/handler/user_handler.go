package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	auth "github.com/smallfire/starfire/internal/service/auth"
	"go.uber.org/zap"
)

// UserHandler 用户管理API处理器
type UserHandler struct {
	authService *auth.AuthService
	logger       *zap.Logger
}

// NewUserHandler 创建用户管理API处理器
func NewUserHandler(authService *auth.AuthService, logger *zap.Logger) *UserHandler {
	return &UserHandler{
		authService: authService,
		logger:       logger,
	}
}

// UpdateUserStatusRequest 更新用户状态请求
type UpdateUserStatusRequest struct {
	IsActive bool `json:"is_active"`
}

// ResetPasswordRequest 重置密码请求
type ResetPasswordRequest struct {
	NewPassword string `json:"new_password" binding:"required"`
}

// ListUsers 获取用户列表
func (h *UserHandler) ListUsers(c *gin.Context) {
	users, err := h.authService.ListUsers()
	if err != nil {
		h.logger.Error("获取用户列表失败", zap.Error(err))
		HandleError(c, http.StatusInternalServerError, err)
		return
	}

	// 清除敏感字段
	type UserSafe struct {
		ID          int        `json:"id"`
		Username    string     `json:"username"`
		Nickname    string     `json:"nickname"`
		Role        string     `json:"role"`
		IsActive    bool       `json:"is_active"`
		LastLoginAt *string    `json:"last_login_at,omitempty"`
		CreatedAt   string     `json:"created_at"`
	}

	safeUsers := make([]UserSafe, 0, len(users))
	for _, u := range users {
		safe := UserSafe{
			ID:        u.ID,
			Username:  u.Username,
			Nickname:  u.Nickname,
			Role:      u.Role,
			IsActive:  u.IsActive,
			CreatedAt: u.CreatedAt.Format("2006-01-02T15:04:05+08:00"),
		}
		if u.LastLoginAt != nil {
			t := u.LastLoginAt.Format("2006-01-02T15:04:05+08:00")
			safe.LastLoginAt = &t
		}
		safeUsers = append(safeUsers, safe)
	}

	HandleSuccess(c, gin.H{"users": safeUsers})
}

// UpdateUserStatus 更新用户状态
func (h *UserHandler) UpdateUserStatus(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		HandleError(c, http.StatusBadRequest, errors.New("无效的用户ID"))
		return
	}

	var req UpdateUserStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		HandleError(c, http.StatusBadRequest, errors.New("请求参数错误"))
		return
	}

	if err := h.authService.UpdateUserStatus(id, req.IsActive); err != nil {
		h.logger.Error("更新用户状态失败", zap.Error(err))
		HandleError(c, http.StatusInternalServerError, err)
		return
	}

	HandleSuccess(c, gin.H{"message": "用户状态已更新"})
}

// ResetPassword 重置用户密码
func (h *UserHandler) ResetPassword(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		HandleError(c, http.StatusBadRequest, errors.New("无效的用户ID"))
		return
	}

	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		HandleError(c, http.StatusBadRequest, errors.New("请求参数错误"))
		return
	}

	if err := h.authService.ResetPassword(id, req.NewPassword); err != nil {
		if errors.Is(err, auth.ErrValidation) {
			HandleError(c, http.StatusBadRequest, err)
			return
		}
		h.logger.Error("重置密码失败", zap.Error(err))
		HandleError(c, http.StatusInternalServerError, err)
		return
	}

	HandleSuccess(c, gin.H{"message": "密码已重置"})
}
