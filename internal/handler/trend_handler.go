package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/smallfire/starfire/internal/repository"
	aiservice "github.com/smallfire/starfire/internal/service/ai"
	marketservice "github.com/smallfire/starfire/internal/service/market"
	"go.uber.org/zap"
)

type TrendHandler struct {
	trendRepo           repository.TrendRepo
	symbolRepo          repository.SymbolRepo
	syncService         *marketservice.SyncService
	pullbackAnalyzer    *aiservice.TrendPullbackAnalyzer
	elliottWaveAnalyzer *aiservice.ElliottWaveAnalyzer
	logger              *zap.Logger
}

func NewTrendHandler(
	trendRepo repository.TrendRepo,
	symbolRepo repository.SymbolRepo,
	syncService *marketservice.SyncService,
	pullbackAnalyzer *aiservice.TrendPullbackAnalyzer,
	elliottWaveAnalyzer *aiservice.ElliottWaveAnalyzer,
	logger *zap.Logger,
) *TrendHandler {
	return &TrendHandler{
		trendRepo:           trendRepo,
		symbolRepo:          symbolRepo,
		syncService:         syncService,
		pullbackAnalyzer:    pullbackAnalyzer,
		elliottWaveAnalyzer: elliottWaveAnalyzer,
		logger:              logger,
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
		// 没有趋势数据是正常情况，返回空而不是错误
		if errors.Is(err, pgx.ErrNoRows) {
			HandleSuccess(c, nil)
			return
		}
		h.logger.Error("获取趋势失败",
			zap.Int("symbol_id", symbolID),
			zap.String("period", period),
			zap.Error(err))
		HandleError(c, http.StatusInternalServerError, err)
		return
	}

	HandleSuccess(c, trend)
}

func (h *TrendHandler) AnalyzePullback(c *gin.Context) {
	if h.pullbackAnalyzer == nil {
		HandleError(c, http.StatusServiceUnavailable, errors.New("AI 趋势回调分析服务未启用"))
		return
	}

	var req aiservice.TrendPullbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		HandleError(c, http.StatusBadRequest, err)
		return
	}
	req.MarketCode = normalizeAnalyzeMarketCode(req.MarketCode)
	req.SymbolCode = normalizeAnalyzeSymbolCode(req.MarketCode, req.SymbolCode)
	if err := h.resolveAnalyzeRequestSymbol(&req.SymbolID, req.MarketCode, req.SymbolCode, req.Period); err != nil {
		HandleError(c, http.StatusBadRequest, err)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer cancel()
	resp, err := h.pullbackAnalyzer.Analyze(ctx, req)
	if err != nil {
		h.logger.Error("AI趋势回调分析失败", zap.String("symbol", req.SymbolCode), zap.String("period", req.Period), zap.Error(err))
		HandleError(c, http.StatusInternalServerError, err)
		return
	}
	HandleSuccess(c, resp)
}

func (h *TrendHandler) AnalyzeElliottWave(c *gin.Context) {
	if h.elliottWaveAnalyzer == nil {
		HandleError(c, http.StatusServiceUnavailable, errors.New("AI艾略特分析服务未启用"))
		return
	}

	var req aiservice.ElliottWaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		HandleError(c, http.StatusBadRequest, err)
		return
	}
	req.MarketCode = normalizeAnalyzeMarketCode(req.MarketCode)
	req.SymbolCode = normalizeAnalyzeSymbolCode(req.MarketCode, req.SymbolCode)
	if err := h.resolveAnalyzeRequestSymbol(&req.SymbolID, req.MarketCode, req.SymbolCode, req.Period); err != nil {
		HandleError(c, http.StatusBadRequest, err)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer cancel()
	resp, err := h.elliottWaveAnalyzer.Analyze(ctx, req)
	if err != nil {
		h.logger.Error("AI艾略特分析失败", zap.String("symbol", req.SymbolCode), zap.String("period", req.Period), zap.Error(err))
		HandleError(c, http.StatusInternalServerError, err)
		return
	}
	HandleSuccess(c, resp)
}

func (h *TrendHandler) resolveAnalyzeRequestSymbol(symbolID *int, marketCode, symbolCode, period string) error {
	if *symbolID > 0 {
		return nil
	}
	if h.symbolRepo == nil {
		return errors.New("symbol_id 不能为空")
	}
	symbol, err := h.symbolRepo.FindByCode(marketCode, symbolCode)
	if err != nil {
		if !strings.Contains(err.Error(), "no rows in result set") || h.syncService == nil {
			return fmt.Errorf("查询标的失败: %w", err)
		}
		symbol, err = h.syncService.EnsureSymbolKlines(marketCode, symbolCode, period)
		if err != nil {
			return fmt.Errorf("自动获取标的数据失败: %w", err)
		}
	} else if h.syncService != nil {
		if syncedSymbol, err := h.syncService.EnsureSymbolKlines(marketCode, symbolCode, period); err != nil {
			h.logger.Warn("分析前刷新观察标的K线失败，继续使用已有数据",
				zap.String("market", marketCode),
				zap.String("symbol", symbolCode),
				zap.String("period", period),
				zap.Error(err))
		} else {
			symbol = syncedSymbol
		}
	}
	if symbol == nil || symbol.ID <= 0 {
		return fmt.Errorf("未找到标的: %s", symbolCode)
	}
	*symbolID = symbol.ID
	return nil
}

func normalizeAnalyzeMarketCode(marketCode string) string {
	code := strings.TrimSpace(marketCode)
	if code == "" {
		return "bybit"
	}
	return strings.ToLower(code)
}

func normalizeAnalyzeSymbolCode(marketCode, symbolCode string) string {
	code := strings.TrimSpace(symbolCode)
	if strings.ToLower(strings.TrimSpace(marketCode)) == "a_stock" {
		code = strings.ToLower(code)
		if strings.HasPrefix(code, "sh") || strings.HasPrefix(code, "sz") || strings.HasPrefix(code, "bj") {
			return code
		}
		if strings.HasPrefix(code, "6") {
			return "sh" + code
		}
		if strings.HasPrefix(code, "8") || strings.HasPrefix(code, "4") {
			return "bj" + code
		}
		return "sz" + code
	}
	return strings.ToUpper(code)
}
