package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/smallfire/starfire/internal/config"
	"github.com/smallfire/starfire/pkg/utils"
)

func main() {
	// 加载配置
	configPath := "/Users/huangjicheng/go/src/github.com/smallfire/config/config.yml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Printf("配置加载失败: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志
	utils.InitLogger(cfg.Log)
	defer func() {
		if utils.Logger != nil {
			_ = utils.Logger.Sync()
		}
	}()
	utils.Info("日志系统初始化成功")

	// 设置 Gin 模式
	if cfg.App.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	// 创建 Gin 实例
	r := gin.Default()

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "healthy",
			"service": cfg.App.Name,
			"version": "1.0.0",
			"time":    time.Now().Unix(),
		})
	})

	// API 版本
	apiV1 := r.Group("/api/v1")
	{
		apiV1.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"code":    0,
				"message": "success",
				"data": gin.H{
					"status": "ok",
				},
				"timestamp": time.Now().Unix(),
			})
		})
	}

	utils.Info("路由初始化完成")

	// 启动服务器
	addr := fmt.Sprintf("%s:%d", cfg.App.Host, cfg.App.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	go func() {
		utils.Info("服务器启动", zap.String("address", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			utils.Fatal("服务器启动失败", zap.Error(err))
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	utils.Info("正在关闭服务器...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		utils.Fatal("服务器强制关闭", zap.Error(err))
	}

	utils.Info("服务器已关闭")
}
