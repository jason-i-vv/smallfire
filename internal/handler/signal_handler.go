package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/smallfire/starfire/internal/repository"
	"go.uber.org/zap"
)

// SignalHandler 信号API处理器
type SignalHandler struct {
	signalRepo repository.SignalRepo
	logger     *zap.Logger
}

// NewSignalHandler 创建信号API处理器
func NewSignalHandler(signalRepo repository.SignalRepo, logger *zap.Logger) *SignalHandler {
	return &SignalHandler{
		signalRepo: signalRepo,
		logger:     logger,
	}
}

// GetSignals 获取所有信号列表
func (h *SignalHandler) GetSignals(c *gin.Context) {
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

	signals, total, err := h.signalRepo.GetHistory(startDate, endDate, page, size)
	if err != nil {
		h.logger.Error("获取信号列表失败", zap.Error(err))
		HandleError(c, http.StatusInternalServerError, err)
		return
	}

	HandleSuccess(c, gin.H{
		"list":  signals,
		"total": total,
		"page":  page,
		"size":  size,
	})
}

// GetSignal 获取指定信号详情
func (h *SignalHandler) GetSignal(c *gin.Context) {
	// 注意：SignalRepo 没有 GetByID 方法，暂时返回空
	HandleSuccess(c, nil)
}

// GetSymbolSignals 获取指定标的的信号
func (h *SignalHandler) GetSymbolSignals(c *gin.Context) {
	symbolID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		HandleError(c, http.StatusBadRequest, err)
		return
	}

	signals, err := h.signalRepo.GetBySymbol(symbolID)
	if err != nil {
		h.logger.Error("获取标的信号失败", zap.Int("symbol_id", symbolID), zap.Error(err))
		HandleError(c, http.StatusInternalServerError, err)
		return
	}

	HandleSuccess(c, signals)
}
