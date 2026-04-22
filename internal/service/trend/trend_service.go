package trend

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/internal/repository"
	"go.uber.org/zap"
)

// Service 趋势服务
type Service struct {
	trendRepo repository.TrendRepo
	logger   *zap.Logger
}

// NewService 创建趋势服务
func NewService(trendRepo repository.TrendRepo, logger *zap.Logger) *Service {
	return &Service{
		trendRepo: trendRepo,
		logger:   logger,
	}
}

// UpdateTrend 计算并更新趋势
// 根据最新K线的EMA计算趋势状态，存储到数据库
func (s *Service) UpdateTrend(symbolID int, period string, klines []models.Kline) error {
	if len(klines) < 30 {
		return nil
	}

	// 获取最新K线
	lastKline := klines[len(klines)-1]

	// 计算趋势
	trendType, strength := s.calculateTrend(klines)

	// 获取当前活跃趋势
	activeTrend, err := s.trendRepo.GetActive(symbolID, period)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		// 只有非 ErrNoRows 的错误才记录
		s.logger.Error("获取活跃趋势失败",
			zap.Int("symbol_id", symbolID),
			zap.String("period", period),
			zap.Error(err))
	}

	// 构建趋势数据
	trend := &models.Trend{
		SymbolID:  symbolID,
		Period:    period,
		TrendType: trendType,
		Strength:  strength,
		Status:    models.TrendStatusActive,
		StartTime: lastKline.OpenTime,
	}

	// 设置 EMA 值
	if lastKline.EMAShort != nil {
		trend.EMAShort = *lastKline.EMAShort
	}
	if lastKline.EMAMedium != nil {
		trend.EMAMedium = *lastKline.EMAMedium
	}
	if lastKline.EMALong != nil {
		trend.EMALong = *lastKline.EMALong
	}

	if activeTrend == nil {
		// 没有活跃趋势，创建新趋势
		if err := s.trendRepo.Create(trend); err != nil {
			s.logger.Error("创建趋势失败",
				zap.Int("symbol_id", symbolID),
				zap.String("period", period),
				zap.Error(err))
			return err
		}
		s.logger.Info("创建新趋势",
			zap.Int("symbol_id", symbolID),
			zap.String("period", period),
			zap.String("trend_type", trendType),
			zap.Int("strength", strength))
	} else if activeTrend.TrendType != trendType {
		// 趋势反转，结束旧趋势，创建新趋势
		activeTrend.Status = models.TrendStatusEnded
		activeTrend.EndTime = &lastKline.OpenTime
		if err := s.trendRepo.Update(activeTrend); err != nil {
			s.logger.Error("更新趋势状态失败",
				zap.Int("symbol_id", symbolID),
				zap.Error(err))
		}

		if err := s.trendRepo.Create(trend); err != nil {
			s.logger.Error("创建趋势失败",
				zap.Int("symbol_id", symbolID),
				zap.String("period", period),
				zap.Error(err))
			return err
		}
		s.logger.Info("趋势反转",
			zap.Int("symbol_id", symbolID),
			zap.String("period", period),
			zap.String("old_trend", activeTrend.TrendType),
			zap.String("new_trend", trendType))
	} else {
		// 趋势类型不变，更新 EMA 值
		activeTrend.EMAShort = trend.EMAShort
		activeTrend.EMAMedium = trend.EMAMedium
		activeTrend.EMALong = trend.EMALong
		activeTrend.Strength = strength
		if err := s.trendRepo.Update(activeTrend); err != nil {
			s.logger.Error("更新趋势失败",
				zap.Int("symbol_id", symbolID),
				zap.Error(err))
			return err
		}
	}

	return nil
}

// calculateTrend 根据K线数据计算趋势
func (s *Service) calculateTrend(klines []models.Kline) (trendType string, strength int) {
	if len(klines) < 30 {
		return models.TrendTypeSideways, 1
	}

	lastKline := klines[len(klines)-1]

	// 优先使用 EMA 计算趋势
	if lastKline.EMAShort != nil && lastKline.EMAMedium != nil && lastKline.EMALong != nil &&
		*lastKline.EMAShort != 0 && *lastKline.EMAMedium != 0 && *lastKline.EMALong != 0 {
		return CalculateFromEMA(*lastKline.EMAShort, *lastKline.EMAMedium, *lastKline.EMALong)
	}

	// EMA 不可用时使用 K 线数据后备计算
	return CalculateFromKlines(klines)
}
