package ai

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
	"time"

	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/internal/repository"
	"go.uber.org/zap"
)

// AIWatchScheduler AI 观察仓调度器
// 实现 market.SyncHook，K 线同步完成后自动分析匹配的观察仓标的
type AIWatchScheduler struct {
	watchRepo  repository.AIWatchTargetRepo
	symbolRepo repository.SymbolRepo
	pullback   *TrendPullbackAnalyzer
	wave       *ElliottWaveAnalyzer
	logger     *zap.Logger
	mu         sync.Mutex
}

// NewAIWatchScheduler 创建观察仓调度器
func NewAIWatchScheduler(
	watchRepo repository.AIWatchTargetRepo,
	symbolRepo repository.SymbolRepo,
	pullback *TrendPullbackAnalyzer,
	wave *ElliottWaveAnalyzer,
	logger *zap.Logger,
) *AIWatchScheduler {
	return &AIWatchScheduler{
		watchRepo:  watchRepo,
		symbolRepo: symbolRepo,
		pullback:   pullback,
		wave:       wave,
		logger:     logger,
	}
}

// OnKlinesSynced 实现 SyncHook 接口，K 线同步完成后触发
func (s *AIWatchScheduler) OnKlinesSynced(symbolID int, symbolCode, marketCode, period string) {
	if !s.mu.TryLock() {
		s.logger.Debug("观察仓调度跳过(正在分析)", zap.String("symbol", symbolCode), zap.String("period", period))
		return
	}
	defer s.mu.Unlock()

	targets, err := s.watchRepo.ListEnabled(marketCode, symbolCode, period)
	if err != nil {
		s.logger.Error("查询观察仓失败", zap.Error(err))
		return
	}
	if len(targets) == 0 {
		return
	}

	for _, target := range targets {
		if !target.Enabled {
			continue
		}
		if target.SymbolID == nil || *target.SymbolID <= 0 {
			sym, err := s.symbolRepo.FindByCode(marketCode, symbolCode)
			if err != nil || sym == nil {
				s.logger.Warn("观察仓标的未找到，跳过", zap.String("symbol", symbolCode))
				continue
			}
			sid := sym.ID
			target.SymbolID = &sid
		}

		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		err := s.analyzeAndSave(ctx, target)
		cancel()
		if err != nil {
			s.logger.Error("观察仓分析失败",
				zap.String("agent", target.AgentType),
				zap.String("symbol", target.SymbolCode),
				zap.String("period", target.Period),
				zap.Error(err))
			target.DataStatus = "error"
			target.ErrorMessage = err.Error()
			_ = s.watchRepo.Upsert(target)
		}
	}
}

// AnalyzeTarget 分析单个标的（对外暴露，供手动分析 API 调用）
func (s *AIWatchScheduler) AnalyzeTarget(ctx context.Context, target *models.AIWatchTarget) error {
	return s.analyzeAndSave(ctx, target)
}

func (s *AIWatchScheduler) analyzeAndSave(ctx context.Context, target *models.AIWatchTarget) error {
	symbolID := 0
	if target.SymbolID != nil {
		symbolID = *target.SymbolID
	}

	var newSteps json.RawMessage

	switch target.AgentType {
	case "trend_pullback":
		resp, err := s.pullback.Analyze(ctx, TrendPullbackRequest{
			SymbolID:   symbolID,
			SymbolCode: target.SymbolCode,
			MarketCode: target.MarketCode,
			Period:     target.Period,
			Direction:  "long",
			Limit:      target.Limit,
			StepLimit:  1,
			SendFeishu: target.SendFeishu,
		})
		if err != nil {
			return err
		}
		newSteps, _ = json.Marshal(resp.Steps)

	case "elliott_wave":
		resp, err := s.wave.Analyze(ctx, ElliottWaveRequest{
			SymbolID:   symbolID,
			SymbolCode: target.SymbolCode,
			MarketCode: target.MarketCode,
			Period:     target.Period,
			Limit:      target.Limit,
			StepLimit:  1,
			SendFeishu: target.SendFeishu,
		})
		if err != nil {
			return err
		}
		newSteps, _ = json.Marshal(resp.Steps)

	default:
		return nil
	}

	// 合并结果
	target.Result = mergeStepsJSON(target.Result, newSteps)
	target.DataStatus = "ready"
	target.ErrorMessage = ""
	now := time.Now().UnixMilli()
	target.LastRunAt = &now

	// 最新 step 判定失效则自动关闭跟踪
	if hasLatestInvalid(target.Result) {
		target.Enabled = false
		s.logger.Info("趋势失效，自动关闭AI跟踪",
			zap.String("agent", target.AgentType),
			zap.String("symbol", target.SymbolCode),
			zap.String("period", target.Period))
	}

	return s.watchRepo.Upsert(target)
}

// hasLatestInvalid 检查最新一条 step 是否为 decision=invalid
func hasLatestInvalid(resultJSON json.RawMessage) bool {
	if len(resultJSON) == 0 {
		return false
	}
	var result struct {
		Steps []json.RawMessage `json:"steps"`
	}
	if json.Unmarshal(resultJSON, &result) != nil || len(result.Steps) == 0 {
		return false
	}
	// steps 按时间升序，取最后一条
	latest := result.Steps[len(result.Steps)-1]
	var step struct {
		Decision string `json:"decision"`
	}
	if json.Unmarshal(latest, &step) != nil {
		return false
	}
	return step.Decision == "invalid"
}

// mergeStepsJSON 合并已有 steps 和新 steps，按 kline_time 去重
func mergeStepsJSON(prevJSON, newStepsJSON json.RawMessage) json.RawMessage {
	prevMap := make(map[int64]json.RawMessage)

	// 解析已有 steps
	if len(prevJSON) > 0 {
		var prev struct {
			Steps []json.RawMessage `json:"steps"`
		}
		if json.Unmarshal(prevJSON, &prev) == nil {
			for _, raw := range prev.Steps {
				var entry struct {
					KlineTime int64 `json:"kline_time"`
				}
				if json.Unmarshal(raw, &entry) == nil {
					prevMap[entry.KlineTime] = raw
				}
			}
		}
	}

	// 解析新 steps 并覆盖
	var newStepList []json.RawMessage
	if json.Unmarshal(newStepsJSON, &newStepList) == nil {
		for _, raw := range newStepList {
			var entry struct {
				KlineTime int64 `json:"kline_time"`
			}
			if json.Unmarshal(raw, &entry) == nil {
				prevMap[entry.KlineTime] = raw
			}
		}
	}

	// 按时间排序
	var keys []int64
	for k := range prevMap {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	merged := make([]json.RawMessage, 0, len(keys))
	for _, k := range keys {
		merged = append(merged, prevMap[k])
	}

	result, _ := json.Marshal(map[string]interface{}{
		"steps":    merged,
		"analyzed": len(merged),
	})
	return result
}
