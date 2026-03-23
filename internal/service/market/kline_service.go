package market

import (
	"time"

	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/internal/repository"
	"go.uber.org/zap"
)

// KlineService K线查询服务
type KlineService struct {
	klineRepo repository.KlineRepo
	factory   *Factory
	logger    *zap.Logger
}

// NewKlineService 创建K线查询服务
func NewKlineService(klineRepo repository.KlineRepo, factory *Factory, logger *zap.Logger) *KlineService {
	return &KlineService{
		klineRepo: klineRepo,
		factory:   factory,
		logger:    logger,
	}
}

// GetKlines 获取K线数据
func (s *KlineService) GetKlines(symbolID int64, period string, startTime, endTime *time.Time, limit int) ([]models.Kline, error) {
	// 1. 先查本地数据库
	klines, err := s.klineRepo.GetBySymbolPeriod(symbolID, period, startTime, endTime, limit)
	if err != nil {
		return nil, err
	}

	// 2. 如果本地数据不足，补充从API获取
	// TODO: 实现API补充数据逻辑

	return klines, nil
}

// GetLatestKline 获取最新K线
func (s *KlineService) GetLatestKline(symbolID int64, period string) (*models.Kline, error) {
	return s.klineRepo.GetLatest(symbolID, period)
}

// GetKlinesWithEMA 获取带EMA指标的K线数据
func (s *KlineService) GetKlinesWithEMA(symbolID int64, period string, limit int) ([]models.Kline, error) {
	return s.klineRepo.GetBySymbolPeriod(symbolID, period, nil, nil, limit)
}

// GetTradingViewKlines 获取用于TradingView图表展示的数据格式
func (s *KlineService) GetTradingViewKlines(symbolID int64, period string, limit int) ([]map[string]interface{}, error) {
	klines, err := s.GetKlinesWithEMA(symbolID, period, limit)
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	for _, k := range klines {
		item := map[string]interface{}{
			"time":   k.OpenTime.Unix(),
			"open":   k.OpenPrice,
			"high":   k.HighPrice,
			"low":    k.LowPrice,
			"close":  k.ClosePrice,
			"volume": k.Volume,
		}
		if k.EMAShort != nil {
			item["ema30"] = *k.EMAShort
		}
		if k.EMAMedium != nil {
			item["ema60"] = *k.EMAMedium
		}
		if k.EMALong != nil {
			item["ema90"] = *k.EMALong
		}
		result = append(result, item)
	}

	return result, nil
}
