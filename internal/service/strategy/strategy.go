package strategy

import (
	"math"
	"time"

	"github.com/smallfire/starfire/internal/models"
)

// ptrTime 将 time.Time 转为 *time.Time
// 通过临时变量确保每调用一次返回独立指针地址
func ptrTime(t time.Time) *time.Time {
	tmp := t // 强制 Go 将 tmp 分配在栈帧稳定位置，避免被复用
	return &tmp
}

// Strategy 策略接口
type Strategy interface {
	// 策略名称
	Name() string

	// 策略类型
	Type() string

	// 是否启用
	Enabled() bool

	// 分析K线数据，生成信号
	Analyze(symbolID int, symbolCode string, period string, klines []models.Kline) ([]models.Signal, error)

	// 获取策略配置
	Config() interface{}
}

// Dependency 策略依赖
type Dependency struct {
	SignalRepo interface {
		Create(signal *models.Signal) error
		GetBySymbol(symbolID int) ([]*models.Signal, error)
		Update(signal *models.Signal) error
	}
	BoxRepo interface {
		GetActiveBySymbol(symbolID int, period string) ([]*models.Box, error)
		Create(box *models.Box) error
		Update(box *models.Box) error
		GetValidBoxes(endDate string, strategy string, period string) ([]*models.Box, error)
	}
	TrendRepo interface {
		GetActive(symbolID int, period string) (*models.Trend, error)
		Create(trend *models.Trend) error
		Update(trend *models.Trend) error
		GetByBatchID(batchID string) ([]*models.Trend, error)
	}
	LevelRepo interface {
		GetActive(symbolID int, period string) ([]*models.KeyLevel, error)
		GetActiveBySource(symbolID int, period string, source string) ([]*models.KeyLevel, error)
		FindActive(symbolID int, period string, levelSubtype string) (*models.KeyLevel, error)
		Create(level *models.KeyLevel) error
		Update(level *models.KeyLevel) error
	}
	LevelV2Repo interface {
		Upsert(symbolID int, period string, resistances, supports []models.KeyLevelEntry) error
		GetBySymbolPeriod(symbolID int, period string) (*models.KeyLevelsV2, error)
	}
	KlineRepo interface {
		GetLatestN(symbolID int, period string, limit int) ([]models.Kline, error)
		GetLatest(symbolID int64, period string) (*models.Kline, error)
	}
	Notifier interface {
		SendSignal(signal *models.Signal) error
	}
}

// CalculateATR 计算 ATR（Average True Range）
func CalculateATR(klines []models.Kline, period int) float64 {
	if period <= 0 {
		period = 14
	}
	if len(klines) < period+1 {
		period = len(klines) - 1
	}
	if period <= 0 {
		return 0
	}

	var trSum float64
	for i := len(klines) - period; i < len(klines); i++ {
		if i == 0 {
			continue
		}
		tr := math.Max(
			klines[i].HighPrice-klines[i].LowPrice,
			math.Max(
				math.Abs(klines[i].HighPrice-klines[i-1].ClosePrice),
				math.Abs(klines[i].LowPrice-klines[i-1].ClosePrice),
			),
		)
		trSum += tr
	}
	return trSum / float64(period)
}

// CalculateSLTP 基于 ATR 计算止盈止损价格
func CalculateSLTP(entryPrice float64, direction string, atr float64, atrMultiplier, riskRewardRatio float64) (stopLoss, takeProfit float64) {
	if atrMultiplier <= 0 {
		atrMultiplier = 2.0
	}
	if riskRewardRatio <= 0 {
		riskRewardRatio = 2.0
	}

	slDistance := atr * atrMultiplier
	minSLDistance := entryPrice * 0.01
	maxSLDistance := entryPrice * 0.05
	if slDistance < minSLDistance {
		slDistance = minSLDistance
	}
	if slDistance > maxSLDistance {
		slDistance = maxSLDistance
	}

	tpDistance := slDistance * riskRewardRatio
	minTPDistance := entryPrice * 0.02
	maxTPDistance := entryPrice * 0.15
	if tpDistance < minTPDistance {
		tpDistance = minTPDistance
	}
	if tpDistance > maxTPDistance {
		tpDistance = maxTPDistance
	}

	if direction == models.DirectionLong {
		stopLoss = entryPrice - slDistance
		takeProfit = entryPrice + tpDistance
	} else {
		stopLoss = entryPrice + slDistance
		takeProfit = entryPrice - tpDistance
	}
	return stopLoss, takeProfit
}
