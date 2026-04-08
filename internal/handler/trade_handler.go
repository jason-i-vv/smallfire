package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/internal/repository"
	"github.com/smallfire/starfire/internal/service/trading"
	"go.uber.org/zap"
)

// TradeHandler 交易跟踪API处理器
type TradeHandler struct {
	trackRepo    repository.TradeTrackRepo
	executor     *trading.TradeExecutor
	statsService *trading.StatisticsService
	logger       *zap.Logger
}

// NewTradeHandler 创建交易跟踪API处理器
func NewTradeHandler(trackRepo repository.TradeTrackRepo, executor *trading.TradeExecutor, statsService *trading.StatisticsService, logger *zap.Logger) *TradeHandler {
	return &TradeHandler{
		trackRepo:    trackRepo,
		executor:     executor,
		statsService: statsService,
		logger:       logger,
	}
}

// GetOpenPositions 获取持仓列表
func (h *TradeHandler) GetOpenPositions(c *gin.Context) {
	tracks, err := h.trackRepo.GetOpenPositions()
	if err != nil {
		h.logger.Error("获取持仓列表失败", zap.Error(err))
		HandleError(c, http.StatusInternalServerError, err)
		return
	}

	HandleSuccess(c, tracks)
}

// GetClosedPositions 获取已平仓记录
func (h *TradeHandler) GetClosedPositions(c *gin.Context) {
	// 解析日期参数
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")

	var startDate, endDate *time.Time

	if startDateStr != "" {
		if t, err := time.Parse("2006-01-02", startDateStr); err == nil {
			startDate = &t
		}
	}

	if endDateStr != "" {
		if t, err := time.Parse("2006-01-02", endDateStr); err == nil {
			t = t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
			endDate = &t
		}
	}

	tracks, err := h.trackRepo.GetClosedTracks(startDate, endDate)
	if err != nil {
		h.logger.Error("获取平仓记录失败", zap.Error(err))
		HandleError(c, http.StatusInternalServerError, err)
		return
	}

	HandleSuccess(c, tracks)
}

// GetTradeHistory 获取交易历史（分页）
func (h *TradeHandler) GetTradeHistory(c *gin.Context) {
	// 解析分页参数
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 {
		page = 1
	}

	size, err := strconv.Atoi(c.DefaultQuery("size", "20"))
	if err != nil || size < 1 || size > 100 {
		size = 20
	}

	// 解析日期参数
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")

	var startDate, endDate time.Time
	now := time.Now()

	if startDateStr != "" {
		if t, err := time.Parse("2006-01-02", startDateStr); err == nil {
			startDate = t
		} else {
			startDate = now.AddDate(0, -1, 0) // 默认一个月前
		}
	} else {
		startDate = now.AddDate(0, -1, 0)
	}

	if endDateStr != "" {
		if t, err := time.Parse("2006-01-02", endDateStr); err == nil {
			endDate = t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
		} else {
			endDate = now
		}
	} else {
		endDate = now
	}

	tracks, total, err := h.trackRepo.GetHistory(startDate, endDate, page, size)
	if err != nil {
		h.logger.Error("获取交易历史失败", zap.Error(err))
		HandleError(c, http.StatusInternalServerError, err)
		return
	}

	HandleSuccess(c, gin.H{
		"list":  tracks,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// GetTradeStats 获取交易统计
func (h *TradeHandler) GetTradeStats(c *gin.Context) {
	// 解析日期参数
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")

	var startDate, endDate *time.Time

	if startDateStr != "" {
		if t, err := time.Parse("2006-01-02", startDateStr); err == nil {
			startDate = &t
		}
	}

	if endDateStr != "" {
		if t, err := time.Parse("2006-01-02", endDateStr); err == nil {
			t = t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
			endDate = &t
		}
	}

	stats, err := h.statsService.GetStatistics(startDate, endDate)
	if err != nil {
		h.logger.Error("获取交易统计失败", zap.Error(err))
		HandleError(c, http.StatusInternalServerError, err)
		return
	}

	HandleSuccess(c, stats)
}

// GetSignalAnalysis 获取信号分析统计
func (h *TradeHandler) GetSignalAnalysis(c *gin.Context) {
	analysis, err := h.statsService.GetSignalAnalysis()
	if err != nil {
		h.logger.Error("获取信号分析失败", zap.Error(err))
		HandleError(c, http.StatusInternalServerError, err)
		return
	}

	HandleSuccess(c, analysis)
}

// GetTradeDetail 获取交易详情
func (h *TradeHandler) GetTradeDetail(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		HandleError(c, http.StatusBadRequest, err)
		return
	}

	track, err := h.trackRepo.GetByID(id)
	if err != nil {
		h.logger.Error("获取交易详情失败", zap.Int("id", id), zap.Error(err))
		HandleError(c, http.StatusInternalServerError, err)
		return
	}

	if track == nil {
		HandleError(c, http.StatusNotFound, nil)
		return
	}

	HandleSuccess(c, track)
}

// ClosePosition 平仓（手动）
func (h *TradeHandler) ClosePosition(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		HandleError(c, http.StatusBadRequest, err)
		return
	}

	var req struct {
		Price float64 `json:"price" binding:"required,gt=0"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		HandleError(c, http.StatusBadRequest, err)
		return
	}

	track, err := h.trackRepo.GetByID(id)
	if err != nil {
		h.logger.Error("查询交易记录失败", zap.Int("id", id), zap.Error(err))
		HandleError(c, http.StatusInternalServerError, err)
		return
	}

	if track == nil || track.Status != models.TrackStatusOpen {
		HandleError(c, http.StatusBadRequest, fmt.Errorf("交易记录不存在或已平仓"))
		return
	}

	if err := h.executor.CloseByManual(track, req.Price); err != nil {
		h.logger.Error("平仓失败", zap.Int("id", id), zap.Error(err))
		HandleError(c, http.StatusInternalServerError, err)
		return
	}

	// 重新查询获取更新后的数据
	updated, _ := h.trackRepo.GetByID(id)
	HandleSuccess(c, updated)
}
