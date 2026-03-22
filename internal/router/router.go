package router

import (
	"github.com/gin-gonic/gin"

	"github.com/smallfire/starfire/internal/config"
	"github.com/smallfire/starfire/internal/database"
)

func SetupRoutes(r *gin.Engine, db *database.DB, cfg *config.Config) {
	// API 版本
	apiV1 := r.Group("/api/v1")

	// 健康检查
	apiV1.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "healthy",
			"service": cfg.App.Name,
		})
	})

	// 这里会添加其他路由组
	// authRouter := apiV1.Group("/auth")
	// marketRouter := apiV1.Group("/markets")
	// symbolRouter := apiV1.Group("/symbols")
	// klineRouter := apiV1.Group("/klines")
	// boxRouter := apiV1.Group("/boxes")
	// trendRouter := apiV1.Group("/trends")
	// signalRouter := apiV1.Group("/signals")
	// tradeRouter := apiV1.Group("/trades")
}
