package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/smallfire/starfire/internal/config"
	"go.uber.org/zap"
)

// StrategyHandler 策略配置API处理器
type StrategyHandler struct {
	config *config.StrategiesConfig
	logger *zap.Logger
}

// NewStrategyHandler 创建策略配置API处理器
func NewStrategyHandler(config *config.StrategiesConfig, logger *zap.Logger) *StrategyHandler {
	return &StrategyHandler{
		config: config,
		logger: logger,
	}
}

// GetStrategies 获取所有策略配置
func (h *StrategyHandler) GetStrategies(c *gin.Context) {
	HandleSuccess(c, gin.H{
		"box":           h.config.Box,
		"trend":         h.config.Trend,
		"key_level":     h.config.KeyLevel,
		"volume_price":  h.config.VolumePrice,
	})
}
