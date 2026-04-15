package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/smallfire/starfire/internal/repository"
	"go.uber.org/zap"
)

type TrendHandler struct {
	trendRepo repository.TrendRepo
	logger    *zap.Logger
}

func NewTrendHandler(trendRepo repository.TrendRepo, logger *zap.Logger) *TrendHandler {
	return &TrendHandler{
		trendRepo: trendRepo,
		logger:    logger,
	}
}

// GetTrendsBySymbol 获取指定标的的趋势状态
func (h *TrendHandler) GetTrendsBySymbol(c *gin.Context) {
	symbolID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		HandleError(c, http.StatusBadRequest, err)
		return
	}

	period := c.DefaultQuery("period", "15m")

	trend, err := h.trendRepo.GetActive(symbolID, period)
	if err != nil {
		h.logger.Error("获取趋势失败",
			zap.Int("symbol_id", symbolID),
			zap.String("period", period),
			zap.Error(err))
		HandleError(c, http.StatusInternalServerError, err)
		return
	}

	HandleSuccess(c, trend)
}
