package trading

import (
	"time"

	"github.com/smallfire/starfire/internal/config"
	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/internal/repository"
)

// RiskManager 风控管理器
type RiskManager struct {
	config        *config.TradingConfig
	trackRepo     repository.TradeTrackRepo
	positionSizer *PositionSizer
}

// RiskCheckResult 风控检查结果
type RiskCheckResult struct {
	Passed bool
	Reason string
}

// NewRiskManager 创建风控管理器实例
func NewRiskManager(cfg *config.TradingConfig, trackRepo repository.TradeTrackRepo, positionSizer *PositionSizer) *RiskManager {
	return &RiskManager{
		config:        cfg,
		trackRepo:     trackRepo,
		positionSizer: positionSizer,
	}
}

// CheckBeforeOpen 开仓前风控检查
func (r *RiskManager) CheckBeforeOpen(signal *models.Signal) *RiskCheckResult {
	// 1. 检查交易开关
	if !r.config.Enabled {
		return &RiskCheckResult{Passed: false, Reason: "交易功能已关闭"}
	}

	// 2. 检查账户回撤
	currentDrawdown := r.calculateDrawdown()
	if currentDrawdown > r.config.MaxDrawdownPercent {
		return &RiskCheckResult{Passed: false, Reason: "账户回撤超限"}
	}

	// 3. 检查每日交易次数
	todayTrades := r.getTodayTradeCount()
	if todayTrades >= r.config.MaxDailyTrades {
		return &RiskCheckResult{Passed: false, Reason: "已达每日最大交易次数"}
	}

	// 4. 检查当前持仓数
	openPositions := r.getOpenPositions()
	if len(openPositions) >= r.config.MaxOpenPositions {
		return &RiskCheckResult{Passed: false, Reason: "已达最大持仓数"}
	}

	// 5. 检查信号有效期
	if r.isSignalExpired(signal) {
		return &RiskCheckResult{Passed: false, Reason: "信号已过期"}
	}

	// 6. 检查标的是否已有持仓
	existingTrack, _ := r.trackRepo.GetOpenBySymbol(signal.SymbolID)
	if existingTrack != nil {
		return &RiskCheckResult{Passed: false, Reason: "该标的已有持仓"}
	}

	return &RiskCheckResult{Passed: true}
}

func (r *RiskManager) calculateDrawdown() float64 {
	initial := r.config.InitialCapital
	current := r.positionSizer.GetCapital()
	if current >= initial {
		return 0
	}
	return (initial - current) / initial
}

func (r *RiskManager) getTodayTradeCount() int {
	today := time.Now().In(time.FixedZone("CST", 8*3600)).Truncate(24 * time.Hour)
	count, _ := r.trackRepo.CountClosedSince(today)
	return count
}

func (r *RiskManager) getOpenPositions() []*models.TradeTrack {
	tracks, _ := r.trackRepo.GetOpenPositions()
	return tracks
}

func (r *RiskManager) isSignalExpired(signal *models.Signal) bool {
	if signal.CreatedAt.IsZero() {
		return false
	}
	expireTime := signal.CreatedAt.Add(time.Duration(r.config.SignalExpireMinutes) * time.Minute)
	return time.Now().After(expireTime)
}
