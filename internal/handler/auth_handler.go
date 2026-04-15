package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	auth "github.com/smallfire/starfire/internal/service/auth"
	"go.uber.org/zap"
)

// AuthHandler 认证API处理器
type AuthHandler struct {
authService *auth.AuthService
	logger       *zap.Logger
}

// NewAuthHandler 创建认证API处理器
func NewAuthHandler(authService *auth.AuthService, logger *zap.Logger) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		logger:       logger,
	}
}

// RegisterRequest 注册请求
type RegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Nickname string `json:"nickname"`
}

// LoginRequest 登录请求
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}

// Register 用户注册
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		HandleError(c, http.StatusBadRequest, auth.ErrValidation)
		return
	}

	result, err := h.authService.Register(req.Username, req.Password, req.Nickname)
	if err != nil {
		if errors.Is(err, auth.ErrDuplicateUsername) {
			HandleError(c, http.StatusConflict, err)
			return
		}
		if errors.Is(err, auth.ErrValidation) {
			HandleError(c, http.StatusBadRequest, err)
			return
		}
		h.logger.Error("注册失败", zap.Error(err))
		HandleError(c, http.StatusInternalServerError, err)
		return
	}

	HandleSuccess(c, gin.H{
		"token": result.Token,
		"user":  result.User,
	})
}

// Login 用户登录
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		HandleError(c, http.StatusBadRequest, auth.ErrInvalidCredentials)
		return
	}

	result, err := h.authService.Login(req.Username, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrUserDisabled) {
			HandleError(c, http.StatusForbidden, err)
			return
		}
		// 统一返回 InvalidCredentials（不暴露用户不存在还是密码错误）
		HandleError(c, http.StatusUnauthorized, auth.ErrInvalidCredentials)
		return
	}

	HandleSuccess(c, gin.H{
		"token": result.Token,
		"user":  result.User,
	})
}

// Me 获取当前用户信息
func (h *AuthHandler) Me(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		// JWT 未启用时中间件不会设置 user_id，返回默认匿名用户
		HandleSuccess(c, gin.H{
			"id":       0,
			"username": "anonymous",
			"role":     "admin",
			"nickname": "匿名用户",
		})
		return
	}

	user, err := h.authService.GetUserByID(userID.(int))
	if err != nil {
		h.logger.Error("获取用户信息失败", zap.Error(err))
		HandleError(c, http.StatusInternalServerError, err)
		return
	}

	HandleSuccess(c, user)
}

// ChangePassword 修改密码
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		HandleError(c, http.StatusBadRequest, errors.New("认证未启用，无法修改密码"))
		return
	}

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		HandleError(c, http.StatusBadRequest, auth.ErrValidation)
		return
	}

	err := h.authService.ChangePassword(userID.(int), req.OldPassword, req.NewPassword)
	if err != nil {
		if errors.Is(err, auth.ErrOldPasswordMismatch) {
			HandleError(c, http.StatusBadRequest, err)
			return
		}
		if errors.Is(err, auth.ErrValidation) {
			HandleError(c, http.StatusBadRequest, err)
			return
		}
		h.logger.Error("修改密码失败", zap.Error(err))
		HandleError(c, http.StatusInternalServerError, err)
		return
	}

	HandleSuccess(c, gin.H{"message": "密码修改成功"})
}
