package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/smallfire/starfire/internal/repository"
	"go.uber.org/zap"
)

// KeyLevelHandler 关键价位API处理器
type KeyLevelHandler struct {
	keyLevelRepo repository.KeyLevelRepo
	logger       *zap.Logger
}

// NewKeyLevelHandler 创建关键价位API处理器
func NewKeyLevelHandler(keyLevelRepo repository.KeyLevelRepo, logger *zap.Logger) *KeyLevelHandler {
	return &KeyLevelHandler{
		keyLevelRepo: keyLevelRepo,
		logger:       logger,
	}
}

// GetKeyLevelsBySymbol 获取指定标的的关键价位
func (h *KeyLevelHandler) GetKeyLevelsBySymbol(c *gin.Context) {
	symbolID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		HandleError(c, http.StatusBadRequest, err)
		return
	}

	period := c.DefaultQuery("period", "15m")

	h.logger.Debug("获取标的的关键价位",
		zap.Int("symbol_id", symbolID),
		zap.String("period", period))

	levels, err := h.keyLevelRepo.GetActive(symbolID, period)
	if err != nil {
		h.logger.Error("获取标的的关键价位失败", zap.Int("symbol_id", symbolID), zap.Error(err))
		HandleError(c, http.StatusInternalServerError, err)
		return
	}

	HandleSuccess(c, gin.H{
		"list":  levels,
		"total": len(levels),
	})
}

// GetAllKeyLevels 获取所有关键价位（分页）
func (h *KeyLevelHandler) GetAllKeyLevels(c *gin.Context) {
	// 解析分页参数
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}

	size, err := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	if err != nil || size < 1 || size > 100 {
		size = 50
	}

	levelType := c.Query("level_type") // "resistance" 或 "support"
	period := c.Query("period")

	h.logger.Debug("获取关键价位列表",
		zap.Int("page", page),
		zap.Int("size", size),
		zap.String("level_type", levelType),
		zap.String("period", period))

	// 暂时只返回所有未突破的关键价位
	// 后续可以添加分页和过滤功能

	HandleSuccess(c, gin.H{
		"list":  []interface{}{},
		"total": 0,
		"page":  page,
		"size":  size,
		"note":  "使用 /symbols/:id/key-levels 接口获取特定标的的关键价位",
	})
}
