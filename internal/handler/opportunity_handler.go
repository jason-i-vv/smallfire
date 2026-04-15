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
	scorer     *scoring.SignalScorer
	aiAnalyzer *aiservice.OpportunityAnalyzer
	aiEnabled  bool
	cooldown   *aiservice.CooldownTracker
	logger     *zap.Logger
}

// NewOpportunityHandler 创建交易机会 handler
func NewOpportunityHandler(
	oppRepo repository.OpportunityRepo,
	scorer *scoring.SignalScorer,
	aiAnalyzer *aiservice.OpportunityAnalyzer,
	aiCfg config.AIConfig,
	cooldown *aiservice.CooldownTracker,
	logger *zap.Logger,
) *OpportunityHandler {
	return &OpportunityHandler{
		oppRepo:    oppRepo,
		scorer:     scorer,
		aiAnalyzer: aiAnalyzer,
		aiEnabled:  aiCfg.Enabled,
		cooldown:   cooldown,
		logger:     logger,
	}
}

// GetOpportunities 获取交易机会列表（按评分排序）
// GET /api/v1/opportunities?status=active&page=1&page_size=20
func (h *OpportunityHandler) GetOpportunities(c *gin.Context) {
	status := c.DefaultQuery("status", "active")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	opportunities, total, err := h.oppRepo.List(status, page, pageSize)
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
// GET /api/v1/opportunities/active
func (h *OpportunityHandler) GetActiveOpportunities(c *gin.Context) {
	opportunities, err := h.oppRepo.GetActive()
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
		"data":    opportunities,
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
