package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/smallfire/starfire/internal/config"
	"github.com/smallfire/starfire/internal/repository"
	aiservice "github.com/smallfire/starfire/internal/service/ai"
	"github.com/smallfire/starfire/internal/service/scoring"
	"go.uber.org/zap"
)

// OpportunityHandler 交易机会 API
type OpportunityHandler struct {
	oppRepo    repository.OpportunityRepo
	trackRepo  repository.TradeTrackRepo
	scorer     *scoring.SignalScorer
	aiAnalyzer *aiservice.OpportunityAnalyzer
	aiEnabled  bool
	cooldown   *aiservice.CooldownTracker
	logger     *zap.Logger
}

// NewOpportunityHandler 创建交易机会 handler
func NewOpportunityHandler(
	oppRepo repository.OpportunityRepo,
	trackRepo repository.TradeTrackRepo,
	scorer *scoring.SignalScorer,
	aiAnalyzer *aiservice.OpportunityAnalyzer,
	aiCfg config.AIConfig,
	cooldown *aiservice.CooldownTracker,
	logger *zap.Logger,
) *OpportunityHandler {
	return &OpportunityHandler{
		oppRepo:    oppRepo,
		trackRepo:  trackRepo,
		scorer:     scorer,
		aiAnalyzer: aiAnalyzer,
		aiEnabled:  aiCfg.Enabled,
		cooldown:   cooldown,
		logger:     logger,
	}
}

// GetOpportunities 获取交易机会列表（按评分排序）
// GET /api/v1/opportunities?status=active&page=1&page_size=20&period=15m&direction=long&symbol=BTC&min_score=50
func (h *OpportunityHandler) GetOpportunities(c *gin.Context) {
	status := c.Query("status")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	period := c.DefaultQuery("period", "")
	direction := c.DefaultQuery("direction", "")
	symbolCode := c.DefaultQuery("symbol", "")
	var minScore *int
	if ms := c.Query("min_score"); ms != "" {
		if v, err := strconv.Atoi(ms); err == nil {
			minScore = &v
		}
	}

	filter := &repository.OpportunityListFilter{
		Status:     status,
		Period:     period,
		Direction:  direction,
		SymbolCode: symbolCode,
		MinScore:   minScore,
		Page:       page,
		PageSize:   pageSize,
	}

	opportunities, total, err := h.oppRepo.List(filter)
	if err != nil {
		h.logger.Error("查询交易机会失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1,
			"message": "查询交易机会失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"items":     opportunities,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// GetOpportunity 获取单个交易机会详情
// GET /api/v1/opportunities/:id
func (h *OpportunityHandler) GetOpportunity(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1,
			"message": "无效的ID",
		})
		return
	}

	opp, err := h.oppRepo.GetByID(id)
	if err != nil {
		h.logger.Error("查询交易机会失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1,
			"message": "查询失败",
		})
		return
	}
	if opp == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    1,
			"message": "交易机会不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    opp,
	})
}

// GetActiveOpportunities 获取所有活跃交易机会（卡片视图用）
// GET /api/v1/opportunities/active?page=1&page_size=100&period=15m&direction=long&symbol=BTC&min_score=50
func (h *OpportunityHandler) GetActiveOpportunities(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "100"))
	period := c.DefaultQuery("period", "")
	direction := c.DefaultQuery("direction", "")
	symbolCode := c.DefaultQuery("symbol", "")
	var minScore *int
	if ms := c.Query("min_score"); ms != "" {
		if v, err := strconv.Atoi(ms); err == nil {
			minScore = &v
		}
	}

	filter := &repository.OpportunityListFilter{
		Status:     "active",
		Period:     period,
		Direction:  direction,
		SymbolCode: symbolCode,
		MinScore:   minScore,
		Page:       page,
		PageSize:   pageSize,
	}

	opportunities, total, err := h.oppRepo.List(filter)
	if err != nil {
		h.logger.Error("查询活跃交易机会失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1,
			"message": "查询失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"items":     opportunities,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// GetOpportunityTrades 获取交易机会关联的交易记录
// GET /api/v1/opportunities/:id/trades
func (h *OpportunityHandler) GetOpportunityTrades(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1,
			"message": "无效的ID",
		})
		return
	}

	// 查询交易机会是否存在
	opp, err := h.oppRepo.GetByID(id)
	if err != nil {
		h.logger.Error("查询交易机会失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1,
			"message": "查询失败",
		})
		return
	}
	if opp == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    1,
			"message": "交易机会不存在",
		})
		return
	}

	// 获取关联的交易记录
	tracks, err := h.trackRepo.GetByOpportunityID(id)
	if err != nil {
		h.logger.Error("查询交易记录失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1,
			"message": "查询失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"opportunity": opp,
			"trades":      tracks,
		},
	})
}

// AIAnalysis AI 分析交易机会
// POST /api/v1/opportunities/:id/ai-analysis
func (h *OpportunityHandler) AIAnalysis(c *gin.Context) {
	// 检查 AI 是否启用
	if !h.aiEnabled || h.aiAnalyzer == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    1,
			"message": "AI 分析服务未启用",
		})
		return
	}

	// 解析 ID
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1,
			"message": "无效的ID",
		})
		return
	}

	// 查询交易机会
	opp, err := h.oppRepo.GetByID(id)
	if err != nil {
		h.logger.Error("查询交易机会失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1,
			"message": "查询失败",
		})
		return
	}
	if opp == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    1,
			"message": "交易机会不存在",
		})
		return
	}

	// 如果已有 AI 判定，直接返回
	if opp.AIJudgment != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data":    opp.AIJudgment,
		})
		return
	}

	// 检查冷却期和每日限额
	canAnalyze, reason := h.cooldown.CanAnalyze(opp.SymbolID)
	if !canAnalyze {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"code":    1,
			"message": reason,
		})
		return
	}

	// 调用 AI 分析
	result, err := h.aiAnalyzer.AnalyzeOpportunity(c.Request.Context(), opp)
	if err != nil {
		h.logger.Error("AI 分析失败", zap.Int("id", id), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1,
			"message": "AI 分析失败: " + err.Error(),
		})
		return
	}

	// 记录调用
	h.cooldown.Record(opp.SymbolID)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    result,
	})
}
