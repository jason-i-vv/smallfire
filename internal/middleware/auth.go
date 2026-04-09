package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/smallfire/starfire/internal/repository"
	"github.com/smallfire/starfire/internal/utils"
)

// AuthMiddleware JWT 认证中间件
func AuthMiddleware(jwtSecret string, userRepo repository.UserRepo) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    10002,
				"message": "未认证，请先登录",
			})
			c.Abort()
			return
		}

		// 提取 Bearer token
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    10002,
				"message": "认证格式错误",
			})
			c.Abort()
			return
		}

		tokenString := parts[1]
		claims, err := utils.ParseToken(tokenString, jwtSecret)
		if err != nil {
			if utils.IsTokenExpired(err) {
				c.JSON(http.StatusUnauthorized, gin.H{
					"code":    10003,
					"message": "认证已过期，请重新登录",
				})
			} else {
				c.JSON(http.StatusUnauthorized, gin.H{
					"code":    10002,
					"message": "认证无效",
				})
			}
			c.Abort()
			return
		}

		// 查询数据库检查用户状态
		user, err := userRepo.GetByID(claims.UserID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    10002,
				"message": "用户不存在",
			})
			c.Abort()
			return
		}

		if !user.IsActive {
			c.JSON(http.StatusForbidden, gin.H{
				"code":    10008,
				"message": "用户已被禁用",
			})
			c.Abort()
			return
		}

		// 将用户信息存入 context
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)

		c.Next()
	}
}

// GetUserID 从 context 获取用户ID
func GetUserID(c *gin.Context) (int, bool) {
	val, exists := c.Get("user_id")
	if !exists {
		return 0, false
	}
	id, ok := val.(int)
	return id, ok
}

// GetRole 从 context 获取用户角色
func GetRole(c *gin.Context) (string, bool) {
	val, exists := c.Get("role")
	if !exists {
		return "", false
	}
	role, ok := val.(string)
	return role, ok
}

// RequireAuth 验证请求是否已认证（不执行 Abort，仅标记）
func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		_, exists := GetUserID(c)
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    10002,
				"message": "未认证",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

var ErrPermissionDenied = errors.New("权限不足")
