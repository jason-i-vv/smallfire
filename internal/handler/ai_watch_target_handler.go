package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/smallfire/starfire/internal/middleware"
	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/internal/repository"
	"go.uber.org/zap"
)

// WatchScheduler 手动分析接口（由 AIWatchScheduler 实现）
type WatchScheduler interface {
	AnalyzeTarget(ctx context.Context, target *models.AIWatchTarget) error
}

type AIWatchTargetHandler struct {
	repo         repository.AIWatchTargetRepo
	scheduler    WatchScheduler
	syncService  interface {
		EnsureSymbolKlines(marketCode, symbolCode, period string) (*models.Symbol, error)
	}
	logger *zap.Logger
}

func NewAIWatchTargetHandler(repo repository.AIWatchTargetRepo, scheduler WatchScheduler, syncService interface {
	EnsureSymbolKlines(marketCode, symbolCode, period string) (*models.Symbol, error)
}, logger *zap.Logger) *AIWatchTargetHandler {
	return &AIWatchTargetHandler{repo: repo, scheduler: scheduler, syncService: syncService, logger: logger}
}

type aiWatchTargetRequest struct {
	SkillName  string          `json:"skill_name"`
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
	skillName := strings.TrimSpace(c.Query("skill_name"))
	if skillName == "" {
		HandleError(c, http.StatusBadRequest, errors.New("skill_name is required"))
		return
	}
	targets, err := h.repo.List(currentUserIDPtr(c), skillName)
	if err != nil {
		h.logger.Error("查询AI观察位失败", zap.String("skill_name", skillName), zap.Error(err))
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
	req.SkillName = strings.TrimSpace(req.SkillName)
	req.MarketCode = strings.TrimSpace(req.MarketCode)
	req.SymbolCode = normalizeAIWatchSymbolCode(req.MarketCode, req.SymbolCode)
	req.Period = strings.TrimSpace(req.Period)
	if req.SkillName == "" || req.MarketCode == "" || req.SymbolCode == "" || req.Period == "" {
		HandleError(c, http.StatusBadRequest, errors.New("skill_name、market_code、symbol_code、period 必填"))
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
		SkillName:    req.SkillName,
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
		h.logger.Error("保存AI观察位失败", zap.String("skill_name", req.SkillName), zap.String("symbol", req.SymbolCode), zap.Error(err))
		HandleError(c, http.StatusInternalServerError, err)
		return
	}

	h.logger.Info("观察仓Upsert成功，开始同步K线",
		zap.String("symbol", req.SymbolCode),
		zap.String("period", req.Period),
		zap.Bool("sync_service_ready", h.syncService != nil))

	// 立即触发标的 K 线同步，确保观察仓能尽快被分析
	if h.syncService != nil {
		if _, err := h.syncService.EnsureSymbolKlines(req.MarketCode, req.SymbolCode, req.Period); err != nil {
			h.logger.Warn("触发K线同步失败，观察仓将在下次同步时被处理",
				zap.String("symbol", req.SymbolCode),
				zap.String("period", req.Period),
				zap.Error(err))
		} else {
			h.logger.Info("K线同步触发成功",
				zap.String("symbol", req.SymbolCode),
				zap.String("period", req.Period))
		}
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

func (h *AIWatchTargetHandler) Analyze(c *gin.Context) {
	h.logger.Info("Analyze 被调用", zap.String("path", c.Request.URL.Path))
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		HandleError(c, http.StatusBadRequest, err)
		return
	}
	if h.scheduler == nil {
		HandleError(c, http.StatusServiceUnavailable, errors.New("AI 分析服务未启用"))
		return
	}

	// 按 ID 直接查询标的（Analyze 按 ID 定位，不限制 user_id）
	target, err := h.repo.GetByIDPublic(id)
	if err != nil {
		HandleError(c, http.StatusInternalServerError, err)
		return
	}
	if target == nil {
		HandleError(c, http.StatusNotFound, errors.New("观察标的未找到"))
		return
	}

	// 立即返回，后台异步执行分析
	// 先将 data_status 标记为 analyzing，让前端知道分析已开始
	target.DataStatus = "analyzing"
	if err := h.repo.Upsert(target); err != nil {
		h.logger.Warn("更新分析状态失败", zap.Int("id", id), zap.Error(err))
	}

	// 异步执行分析
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		if err := h.scheduler.AnalyzeTarget(ctx, target); err != nil {
			h.logger.Error("异步分析失败", zap.Int("id", id), zap.Error(err))
			target.DataStatus = "error"
			target.ErrorMessage = err.Error()
			_ = h.repo.Upsert(target)
		}
	}()

	HandleSuccess(c, gin.H{
		"id":          id,
		"data_status": "analyzing",
		"message":     "分析已提交，结果将通过轮询获取",
	})
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
