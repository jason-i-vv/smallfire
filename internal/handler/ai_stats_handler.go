package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	aiservice "github.com/smallfire/starfire/internal/service/ai"
	"go.uber.org/zap"
)

// AIStatsHandler AI 统计 API 处理器
type AIStatsHandler struct {
	statsSvc *aiservice.AIStatsService
	logger   *zap.Logger
}

// NewAIStatsHandler 创建 AI 统计处理器
func NewAIStatsHandler(statsSvc *aiservice.AIStatsService, logger *zap.Logger) *AIStatsHandler {
	return &AIStatsHandler{statsSvc: statsSvc, logger: logger}
}

func (h *AIStatsHandler) parseDateRange(c *gin.Context) (startDate, endDate *time.Time) {
	if s := c.Query("start_date"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			startDate = &t
		}
	}
	if s := c.Query("end_date"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			t = t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
			endDate = &t
		}
	}
	return
}

// GetDailyCallStats 每日调用统计
func (h *AIStatsHandler) GetDailyCallStats(c *gin.Context) {
	startDate, endDate := h.parseDateRange(c)
	data, err := h.statsSvc.GetDailyCallStats(startDate, endDate)
	if err != nil {
		h.logger.Error("获取每日AI调用统计失败", zap.Error(err))
		HandleError(c, http.StatusInternalServerError, err)
		return
	}
	HandleSuccess(c, data)
}

// GetOverview AI 分析概览
func (h *AIStatsHandler) GetOverview(c *gin.Context) {
	startDate, endDate := h.parseDateRange(c)
	data, err := h.statsSvc.GetOverview(startDate, endDate)
	if err != nil {
		h.logger.Error("获取AI概览失败", zap.Error(err))
		HandleError(c, http.StatusInternalServerError, err)
		return
	}
	HandleSuccess(c, data)
}

// GetAccuracyAnalysis AI 准确率分析
func (h *AIStatsHandler) GetAccuracyAnalysis(c *gin.Context) {
	startDate, endDate := h.parseDateRange(c)
	data, err := h.statsSvc.GetAccuracyAnalysis(startDate, endDate)
	if err != nil {
		h.logger.Error("获取AI准确率分析失败", zap.Error(err))
		HandleError(c, http.StatusInternalServerError, err)
		return
	}
	HandleSuccess(c, data)
}

// GetDirectionStats 按方向统计
func (h *AIStatsHandler) GetDirectionStats(c *gin.Context) {
	startDate, endDate := h.parseDateRange(c)
	data, err := h.statsSvc.GetDirectionStats(startDate, endDate)
	if err != nil {
		h.logger.Error("获取AI方向统计失败", zap.Error(err))
		HandleError(c, http.StatusInternalServerError, err)
		return
	}
	HandleSuccess(c, data)
}

// GetConfidenceAnalysis 置信度分析
func (h *AIStatsHandler) GetConfidenceAnalysis(c *gin.Context) {
	startDate, endDate := h.parseDateRange(c)
	data, err := h.statsSvc.GetConfidenceAnalysis(startDate, endDate)
	if err != nil {
		h.logger.Error("获取AI置信度分析失败", zap.Error(err))
		HandleError(c, http.StatusInternalServerError, err)
		return
	}
	HandleSuccess(c, data)
}
