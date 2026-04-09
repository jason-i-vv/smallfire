package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RequireRole 角色权限中间件
func RequireRole(roles ...string) gin.HandlerFunc {
	roleSet := make(map[string]bool, len(roles))
	for _, r := range roles {
		roleSet[r] = true
	}

	return func(c *gin.Context) {
		userRole, exists := GetRole(c)
		if !exists || !roleSet[userRole] {
			c.JSON(http.StatusForbidden, gin.H{
				"code":    10004,
				"message": "权限不足",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
