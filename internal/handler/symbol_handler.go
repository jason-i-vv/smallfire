package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/smallfire/starfire/internal/repository"
	"go.uber.org/zap"
)

// SymbolHandler 交易标的API处理器
type SymbolHandler struct {
	symbolRepo repository.SymbolRepo
	klineRepo  repository.KlineRepo
	logger     *zap.Logger
}

// NewSymbolHandler 创建交易标的API处理器
func NewSymbolHandler(symbolRepo repository.SymbolRepo, klineRepo repository.KlineRepo, logger *zap.Logger) *SymbolHandler {
	return &SymbolHandler{
		symbolRepo: symbolRepo,
		klineRepo:  klineRepo,
		logger:     logger,
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

	klines, err := h.klineRepo.GetBySymbolPeriod(int64(symbolID), period, nil, nil, limit)
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

	klines, err := h.klineRepo.GetBySymbolPeriod(int64(symbolID), period, nil, nil, limit)
	if err != nil {
		h.logger.Error("获取K线数据失败", zap.Int("symbol_id", symbolID), zap.String("period", period), zap.Error(err))
		HandleError(c, http.StatusInternalServerError, err)
		return
	}

	HandleSuccess(c, klines)
}
