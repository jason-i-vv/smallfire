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
	keyLevelV2Repo repository.KeyLevelV2Repo
	logger         *zap.Logger
}

// NewKeyLevelHandler 创建关键价位API处理器
func NewKeyLevelHandler(keyLevelV2Repo repository.KeyLevelV2Repo, logger *zap.Logger) *KeyLevelHandler {
	return &KeyLevelHandler{
		keyLevelV2Repo: keyLevelV2Repo,
		logger:         logger,
	}
}

// GetKeyLevelsBySymbol 获取指定标的的关键价位
func (h *KeyLevelHandler) GetKeyLevelsBySymbol(c *gin.Context) {
	symbolID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		HandleError(c, http.StatusBadRequest, err)
		return
	}

	period := c.DefaultQuery("period", "1h")

	h.logger.Debug("获取标的的关键价位",
		zap.Int("symbol_id", symbolID),
		zap.String("period", period))

	levels, err := h.keyLevelV2Repo.GetBySymbolPeriod(symbolID, period)
	if err != nil {
		h.logger.Error("获取标的的关键价位失败", zap.Int("symbol_id", symbolID), zap.Error(err))
		HandleError(c, http.StatusInternalServerError, err)
		return
	}

	if levels == nil {
		HandleSuccess(c, gin.H{
			"resistances": []interface{}{},
			"supports":    []interface{}{},
		})
		return
	}

	HandleSuccess(c, gin.H{
		"resistances": levels.Resistances,
		"supports":    levels.Supports,
		"updated_at":  levels.UpdatedAt,
	})
}

// GetAllKeyLevels 获取所有关键价位（分页）
func (h *KeyLevelHandler) GetAllKeyLevels(c *gin.Context) {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}

	size, err := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	if err != nil || size < 1 || size > 100 {
		size = 50
	}

	HandleSuccess(c, gin.H{
		"list":  []interface{}{},
		"total": 0,
		"page":  page,
		"size":  size,
		"note":  "使用 /symbols/:id/key-levels 接口获取特定标的的关键价位",
	})
}
