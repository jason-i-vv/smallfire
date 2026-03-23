package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/smallfire/starfire/internal/repository"
	"go.uber.org/zap"
)

// MarketHandler 市场API处理器
type MarketHandler struct {
	marketRepo repository.MarketRepo
	logger     *zap.Logger
}

// NewMarketHandler 创建市场API处理器
func NewMarketHandler(marketRepo repository.MarketRepo, logger *zap.Logger) *MarketHandler {
	return &MarketHandler{
		marketRepo: marketRepo,
		logger:     logger,
	}
}

// GetMarkets 获取所有市场列表
func (h *MarketHandler) GetMarkets(c *gin.Context) {
	markets, err := h.marketRepo.FindEnabled()
	if err != nil {
		h.logger.Error("获取市场列表失败", zap.Error(err))
		HandleError(c, http.StatusInternalServerError, err)
		return
	}
	HandleSuccess(c, markets)
}

// GetMarket 获取指定市场详情
func (h *MarketHandler) GetMarket(c *gin.Context) {
	marketCode := c.Param("market_code")
	market, err := h.marketRepo.FindByCode(marketCode)
	if err != nil {
		h.logger.Error("获取市场详情失败", zap.String("market_code", marketCode), zap.Error(err))
		HandleError(c, http.StatusInternalServerError, err)
		return
	}
	HandleSuccess(c, market)
}
