package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/smallfire/starfire/internal/models"
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

	size, err := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if err != nil || size < 1 || size > 100 {
		size = 20
	}

	// 解析筛选参数
	query := &models.SignalQuery{
		Market:   c.Query("market"),
		Status:   c.DefaultQuery("status", "pending"),
		Page:     page,
		PageSize: size,
	}

	// 解析标的代码 (symbolCode)
	symbolCode := c.Query("symbolCode")
	if symbolCode != "" {
		query.SymbolCode = symbolCode
	}

	// 解析策略来源 (sourceType)
	sourceType := c.Query("sourceType")
	if sourceType != "" {
		query.SourceType = sourceType
	}

	// 解析信号类型 (注意前端传的是signalType，后端字段是signal_type)
	signalType := c.Query("signalType")
	if signalType != "" {
		query.SignalType = signalType
	}

	// 解析方向
	direction := c.Query("direction")
	if direction != "" {
		query.Direction = direction
	}

	// 解析强度
	strengthStr := c.Query("strength")
	if strengthStr != "" {
		if strength, err := strconv.Atoi(strengthStr); err == nil {
			query.Strength = &strength
		}
	}

	// 解析日期参数
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")

	now := time.Now()
	if startDateStr != "" {
		if t, err := time.Parse("2006-01-02", startDateStr); err == nil {
			query.StartDate = &t
		}
	}
	if endDateStr != "" {
		if t, err := time.Parse("2006-01-02", endDateStr); err == nil {
			endTime := t.Add(23*time.Hour + 59*59*time.Second)
			query.EndDate = &endTime
		}
	}

	// 如果没有指定日期范围，默认查询一个月内的数据
	if query.StartDate == nil && query.EndDate == nil {
		defaultStart := now.AddDate(0, -1, 0)
		query.StartDate = &defaultStart
		query.EndDate = &now
	}

	// 执行查询
	signals, total, err := h.signalRepo.Query(query)
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
	signalID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		HandleError(c, http.StatusBadRequest, err)
		return
	}

	signal, err := h.signalRepo.GetByID(signalID)
	if err != nil {
		h.logger.Error("获取信号详情失败", zap.Int("signal_id", signalID), zap.Error(err))
		HandleError(c, http.StatusInternalServerError, err)
		return
	}

	HandleSuccess(c, signal)
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

// GetSignalCounts 获取信号数量统计
func (h *SignalHandler) GetSignalCounts(c *gin.Context) {
	// 获取各市场的信号数量
	marketCounts := make(map[string]int)
	markets := []string{"", "bybit", "a_stock", "us_stock"}
	for _, market := range markets {
		count, err := h.signalRepo.CountByMarket(market)
		if err != nil {
			h.logger.Warn("统计市场信号数量失败", zap.String("market", market), zap.Error(err))
			count = 0
		}
		key := market
		if market == "" {
			key = "total"
		}
		marketCounts[key] = count
	}

	// 获取各信号类型的数量
	signalTypeCounts := make(map[string]int)
	signalTypes := []string{"", "box_breakout", "box_breakdown", "trend_retracement", "resistance_break", "volume_price_rise", "volume_price_fall", "price_surge", "upper_wick_reversal", "lower_wick_reversal", "fake_breakout_upper", "fake_breakout_lower"}
	for _, signalType := range signalTypes {
		count, err := h.signalRepo.CountBySignalType(signalType)
		if err != nil {
			h.logger.Warn("统计信号类型数量失败", zap.String("signal_type", signalType), zap.Error(err))
			count = 0
		}
		key := signalType
		if signalType == "" {
			key = "total"
		}
		signalTypeCounts[key] = count
	}

	// 获取各策略来源的数量
	sourceTypeCounts := make(map[string]int)
	sourceTypes := []string{"", "box", "trend", "key_level", "volume", "wick"}
	for _, sourceType := range sourceTypes {
		count, err := h.signalRepo.CountBySourceType(sourceType)
		if err != nil {
			h.logger.Warn("统计策略来源信号数量失败", zap.String("source_type", sourceType), zap.Error(err))
			count = 0
		}
		key := sourceType
		if sourceType == "" {
			key = "total"
		}
		sourceTypeCounts[key] = count
	}

	HandleSuccess(c, gin.H{
		"market":       marketCounts,
		"signal_type":  signalTypeCounts,
		"source_type":  sourceTypeCounts,
	})
}
