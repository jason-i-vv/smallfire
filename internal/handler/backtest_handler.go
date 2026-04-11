package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/internal/service/backtest"
	"go.uber.org/zap"
)

// BacktestHandler 回测API处理器
type BacktestHandler struct {
	backtestService *backtest.BacktestService
	logger          *zap.Logger
}

// NewBacktestHandler 创建回测API处理器
func NewBacktestHandler(backtestService *backtest.BacktestService, logger *zap.Logger) *BacktestHandler {
	return &BacktestHandler{
		backtestService: backtestService,
		logger:          logger,
	}
}

// RunBacktest 执行回测
// @Summary 执行回测
// @Description 对指定标的、时间范围、周期执行策略回测
// @Tags backtest
// @Accept json
// @Produce json
// @Param request body models.BacktestRequest true "回测请求参数"
// @Success 200 {object} Response{data=models.BacktestResponse}
// @Failure 400 {object} Response
// @Failure 500 {object} Response
// @Router /api/v1/backtest [post]
func (h *BacktestHandler) RunBacktest(c *gin.Context) {
	var req models.BacktestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		HandleError(c, 400, err)
		return
	}

	// 验证参数
	if err := h.validateRequest(&req); err != nil {
		HandleError(c, 400, err)
		return
	}

	h.logger.Info("开始回测",
		zap.String("symbol", req.SymbolCode),
		zap.String("market", req.MarketCode),
		zap.String("period", req.Period),
		zap.String("strategy", req.StrategyType),
		zap.String("start_time", req.StartTime),
		zap.String("end_time", req.EndTime))

	result, err := h.backtestService.RunBacktest(&req)
	if err != nil {
		h.logger.Error("回测失败", zap.Error(err))
		HandleError(c, 500, err)
		return
	}

	HandleSuccess(c, result)
}

// GetSupportedStrategies 获取支持的策略列表
// @Summary 获取支持的策略列表
// @Description 获取当前系统支持的回测策略列表
// @Tags backtest
// @Produce json
// @Success 200 {object} Response{data=[]map[string]string}
// @Router /api/v1/backtest/strategies [get]
func (h *BacktestHandler) GetSupportedStrategies(c *gin.Context) {
	strategies := h.backtestService.GetSupportedStrategies()
	HandleSuccess(c, strategies)
}

// GetSupportedPeriods 获取支持的周期列表
// @Summary 获取支持的周期列表
// @Description 获取当前系统支持的K线周期列表
// @Tags backtest
// @Produce json
// @Success 200 {object} Response{data=[]string}
// @Router /api/v1/backtest/periods [get]
func (h *BacktestHandler) GetSupportedPeriods(c *gin.Context) {
	periods := h.backtestService.GetSupportedPeriods()
	HandleSuccess(c, periods)
}

// validateRequest 验证请求参数
func (h *BacktestHandler) validateRequest(req *models.BacktestRequest) error {
	// 验证市场代码
	validMarkets := map[string]bool{
		"bybit":    true,
		"a_stock":  true,
		"us_stock": true,
	}
	if !validMarkets[req.MarketCode] {
		return &ValidationError{Field: "market_code", Message: "不支持的市场代码"}
	}

	// 验证周期
	validPeriods := map[string]bool{
		"15m": true,
		"1h":  true,
		"1d":  true,
	}
	if !validPeriods[req.Period] {
		return &ValidationError{Field: "period", Message: "不支持的周期，支持: 15m, 1h, 1d"}
	}

	// 验证策略类型
	validStrategies := map[string]bool{
		"box":          true,
		"trend":        true,
		"key_level":    true,
		"volume_price": true,
		"wick":         true,
		"candlestick":  true,
	}
	if !validStrategies[req.StrategyType] {
		return &ValidationError{Field: "strategy_type", Message: "不支持的策略类型"}
	}

	return nil
}

// ValidationError 验证错误
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}
