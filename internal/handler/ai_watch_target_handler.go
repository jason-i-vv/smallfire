package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/smallfire/starfire/internal/middleware"
	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/internal/repository"
	"go.uber.org/zap"
)

type AIWatchTargetHandler struct {
	repo   repository.AIWatchTargetRepo
	logger *zap.Logger
}

func NewAIWatchTargetHandler(repo repository.AIWatchTargetRepo, logger *zap.Logger) *AIWatchTargetHandler {
	return &AIWatchTargetHandler{repo: repo, logger: logger}
}

type aiWatchTargetRequest struct {
	AgentType  string          `json:"agent_type"`
	MarketCode string          `json:"market_code"`
	SymbolCode string          `json:"symbol_code"`
	SymbolID   *int            `json:"symbol_id"`
	Period     string          `json:"period"`
	Limit      int             `json:"limit"`
	SendFeishu bool            `json:"send_feishu"`
	Enabled    bool            `json:"enabled"`
	DataStatus string          `json:"data_status"`
	Error      string          `json:"error"`
	LastRunAt  *int64          `json:"last_run_at"`
	Result     json.RawMessage `json:"result"`
}

func (h *AIWatchTargetHandler) List(c *gin.Context) {
	agentType := strings.TrimSpace(c.Query("agent_type"))
	if agentType == "" {
		HandleError(c, http.StatusBadRequest, errors.New("agent_type is required"))
		return
	}
	targets, err := h.repo.List(currentUserIDPtr(c), agentType)
	if err != nil {
		h.logger.Error("查询AI观察位失败", zap.String("agent_type", agentType), zap.Error(err))
		HandleError(c, http.StatusInternalServerError, err)
		return
	}
	HandleSuccess(c, targets)
}

func (h *AIWatchTargetHandler) Upsert(c *gin.Context) {
	var req aiWatchTargetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		HandleError(c, http.StatusBadRequest, err)
		return
	}
	req.AgentType = strings.TrimSpace(req.AgentType)
	req.MarketCode = strings.TrimSpace(req.MarketCode)
	req.SymbolCode = normalizeAIWatchSymbolCode(req.MarketCode, req.SymbolCode)
	req.Period = strings.TrimSpace(req.Period)
	if req.AgentType == "" || req.MarketCode == "" || req.SymbolCode == "" || req.Period == "" {
		HandleError(c, http.StatusBadRequest, errors.New("agent_type、market_code、symbol_code、period 必填"))
		return
	}
	if req.Limit <= 0 {
		req.Limit = 120
	}
	if req.DataStatus == "" {
		req.DataStatus = "pending"
	}

	target := &models.AIWatchTarget{
		UserID:       currentUserIDPtr(c),
		AgentType:    req.AgentType,
		MarketCode:   req.MarketCode,
		SymbolCode:   req.SymbolCode,
		SymbolID:     req.SymbolID,
		Period:       req.Period,
		Limit:        req.Limit,
		SendFeishu:   req.SendFeishu,
		Enabled:      req.Enabled,
		DataStatus:   req.DataStatus,
		ErrorMessage: req.Error,
		LastRunAt:    req.LastRunAt,
		Result:       req.Result,
	}
	if err := h.repo.Upsert(target); err != nil {
		h.logger.Error("保存AI观察位失败", zap.String("agent_type", req.AgentType), zap.String("symbol", req.SymbolCode), zap.Error(err))
		HandleError(c, http.StatusInternalServerError, err)
		return
	}
	HandleSuccess(c, target)
}

func (h *AIWatchTargetHandler) Delete(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		HandleError(c, http.StatusBadRequest, err)
		return
	}
	if err := h.repo.Delete(currentUserIDPtr(c), id); err != nil {
		h.logger.Error("删除AI观察位失败", zap.Int("id", id), zap.Error(err))
		HandleError(c, http.StatusInternalServerError, err)
		return
	}
	HandleSuccess(c, gin.H{"deleted": true})
}

func currentUserIDPtr(c *gin.Context) *int {
	userID, ok := middleware.GetUserID(c)
	if !ok || userID <= 0 {
		return nil
	}
	return &userID
}

func normalizeAIWatchSymbolCode(marketCode, symbolCode string) string {
	code := strings.TrimSpace(symbolCode)
	market := strings.ToLower(strings.TrimSpace(marketCode))
	if market == "a_stock" {
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
