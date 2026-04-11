package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/smallfire/starfire/internal/repository"
	"github.com/smallfire/starfire/internal/service/market"
	"go.uber.org/zap"
)

// SymbolHandler 交易标的API处理器
type SymbolHandler struct {
	symbolRepo   repository.SymbolRepo
	klineRepo    repository.KlineRepo
	klineService *market.KlineService
	logger       *zap.Logger
}

// NewSymbolHandler 创建交易标的API处理器
func NewSymbolHandler(symbolRepo repository.SymbolRepo, klineRepo repository.KlineRepo, klineService *market.KlineService, logger *zap.Logger) *SymbolHandler {
	return &SymbolHandler{
		symbolRepo:   symbolRepo,
		klineRepo:    klineRepo,
		klineService: klineService,
		logger:       logger,
	}
}

// GetSymbols 获取所有交易标的
func (h *SymbolHandler) GetSymbols(c *gin.Context) {
	// 这里简化处理，实际应该支持分页和筛选
	// 暂时获取所有标的
	// 由于 SymbolRepo 没有 GetAll 方法，我们需要通过市场来获取
	// 先获取所有市场代码，然后逐个获取
	// 注意：这种方法效率较低，但为了演示API结构暂时这样实现

	// 暂时返回空数据，后续需要根据实际需求实现
	HandleSuccess(c, []interface{}{})
}

// ResolveSymbol 通过 symbol_code 查找 symbol（不区分市场）
func (h *SymbolHandler) ResolveSymbol(c *gin.Context) {
	symbolCode := c.Query("symbol_code")
	if symbolCode == "" {
		HandleError(c, http.StatusBadRequest, errors.New("symbol_code is required"))
		return
	}

	markets := []string{"bybit", "a_stock", "us_stock"}
	for _, market := range markets {
		symbol, err := h.symbolRepo.FindByCode(market, symbolCode)
		if err == nil && symbol != nil {
			HandleSuccess(c, symbol)
			return
		}
	}

	HandleError(c, http.StatusNotFound, fmt.Errorf("未找到标的: %s", symbolCode))
}

// GetMarketSymbols 获取指定市场的交易标的
func (h *SymbolHandler) GetMarketSymbols(c *gin.Context) {
	marketCode := c.Param("market_code")
	symbols, err := h.symbolRepo.GetTrackingByMarket(marketCode)
	if err != nil {
		h.logger.Error("获取市场交易标的失败", zap.String("market_code", marketCode), zap.Error(err))
		HandleError(c, http.StatusInternalServerError, err)
		return
	}
	HandleSuccess(c, symbols)
}

// GetSymbolKlines 获取指定标的的K线数据
func (h *SymbolHandler) GetSymbolKlines(c *gin.Context) {
	symbolID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		HandleError(c, http.StatusBadRequest, err)
		return
	}

	period := c.DefaultQuery("period", "15m")
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "100"))
	if err != nil || limit <= 0 || limit > 1000 {
		limit = 100
	}

	klines, err := h.klineService.GetKlines(int64(symbolID), period, nil, nil, limit)
	if err != nil {
		h.logger.Error("获取K线数据失败", zap.Int("symbol_id", symbolID), zap.String("period", period), zap.Error(err))
		HandleError(c, http.StatusInternalServerError, err)
		return
	}

	HandleSuccess(c, klines)
}

// GetKlines 通用K线数据获取接口
func (h *SymbolHandler) GetKlines(c *gin.Context) {
	symbolIDStr := c.Query("symbol_id")
	if symbolIDStr == "" {
		HandleError(c, http.StatusBadRequest, errors.New("symbol_id is required"))
		return
	}

	symbolID, err := strconv.Atoi(symbolIDStr)
	if err != nil {
		HandleError(c, http.StatusBadRequest, errors.New("invalid symbol_id"))
		return
	}

	period := c.DefaultQuery("period", "15m")
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "100"))
	if err != nil || limit <= 0 || limit > 1000 {
		limit = 100
	}

	// 支持按时间范围获取K线（使用时间戳）
	var startTime, endTime *time.Time
	if startStr := c.Query("start_time"); startStr != "" {
		// 支持两种格式：1) 时间戳数字字符串 2) ISO8601字符串
		if unixSec, err := strconv.ParseInt(startStr, 10, 64); err == nil {
			// 时间戳（秒级）
			t := time.Unix(unixSec, 0)
			startTime = &t
		} else if t, err := time.Parse(time.RFC3339, startStr); err == nil {
			// ISO8601格式（兼容）
			startTime = &t
		}
	}
	if endStr := c.Query("end_time"); endStr != "" {
		if unixSec, err := strconv.ParseInt(endStr, 10, 64); err == nil {
			// 时间戳（秒级）
			t := time.Unix(unixSec, 0)
			endTime = &t
		} else if t, err := time.Parse(time.RFC3339, endStr); err == nil {
			// ISO8601格式（兼容）
			endTime = &t
		}
	}

	klines, err := h.klineService.GetKlines(int64(symbolID), period, startTime, endTime, limit)
	if err != nil {
		h.logger.Error("获取K线数据失败", zap.Int("symbol_id", symbolID), zap.String("period", period), zap.Error(err))
		HandleError(c, http.StatusInternalServerError, err)
		return
	}

	HandleSuccess(c, klines)
}
