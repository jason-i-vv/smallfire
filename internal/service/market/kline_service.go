package market

import (
	"fmt"
	"sync"
	"time"

	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/internal/repository"
	"go.uber.org/zap"
)

// KlineService K线查询服务
type KlineService struct {
	klineRepo  repository.KlineRepo
	symbolRepo repository.SymbolRepo
	factory    *Factory
	logger     *zap.Logger
	sfMu      sync.Mutex
	sfInFlight map[string]struct{}
}

func newSingleflightMap() map[string]struct{} {
	return make(map[string]struct{})
}

// NewKlineService 创建K线查询服务
func NewKlineService(klineRepo repository.KlineRepo, symbolRepo repository.SymbolRepo, factory *Factory, logger *zap.Logger) *KlineService {
	return &KlineService{
		klineRepo:  klineRepo,
		symbolRepo: symbolRepo,
		factory:    factory,
		logger:     logger,
		sfInFlight: newSingleflightMap(),
	}
}

// GetKlines 获取K线数据，数据不足时触发后台同步，立即返回DB数据
func (s *KlineService) GetKlines(symbolID int64, period string, startTime, endTime *time.Time, limit int) ([]models.Kline, error) {
	// 1. 先查本地数据库（总是快速返回）
	dbKlines, err := s.klineRepo.GetBySymbolPeriod(symbolID, period, startTime, endTime, limit)
	if err != nil {
		return nil, err
	}

	// 2. 判断是否需要 API 补充
	needSupplement, fetchStart, fetchEnd := s.needSupplement(dbKlines, period, startTime, endTime, limit)
	if !needSupplement {
		return dbKlines, nil
	}

	// 3. 数据不足，触发后台同步（不阻塞，立即返回DB数据）
	go s.syncKlinesBackground(symbolID, period, fetchStart, fetchEnd)

	return dbKlines, nil
}

// syncKlinesBackground 后台同步K线数据到本地数据库
func (s *KlineService) syncKlinesBackground(symbolID int64, period string, fetchStart, fetchEnd time.Time) {
	// 并发去重：防止同一标的并发触发多次 API 同步
	dedupKey := fmt.Sprintf("%d:%s", symbolID, period)
	s.sfMu.Lock()
	if _, inFlight := s.sfInFlight[dedupKey]; inFlight {
		s.sfMu.Unlock()
		return
	}
	s.sfInFlight[dedupKey] = struct{}{}
	s.sfMu.Unlock()
	defer func() {
		s.sfMu.Lock()
		delete(s.sfInFlight, dedupKey)
		s.sfMu.Unlock()
	}()

	// 获取 symbol 信息
	symbol, err := s.symbolRepo.GetByID(int(symbolID))
	if err != nil {
		s.logger.Warn("获取标的信息失败，跳过后台同步",
			zap.Int64("symbol_id", symbolID), zap.Error(err))
		return
	}

	// 获取对应的 fetcher
	fetcher, ok := s.factory.GetFetcher(symbol.MarketCode)
	if !ok || fetcher == nil {
		s.logger.Warn("不支持的市场，跳过后台同步",
			zap.String("market_code", symbol.MarketCode))
		return
	}

	// 从交易所 API 获取数据
	apiPeriod := MapPeriod(symbol.MarketCode, period)
	apiKlines, err := fetcher.FetchKlinesByTimeRange(symbol.SymbolCode, apiPeriod, fetchStart, fetchEnd)
	if err != nil {
		s.logger.Warn("后台同步：从交易所API获取K线数据失败",
			zap.String("symbol", symbol.SymbolCode),
			zap.String("period", period),
			zap.Error(err))
		return
	}

	if len(apiKlines) == 0 {
		s.logger.Info("后台同步：无新数据",
			zap.String("symbol", symbol.SymbolCode),
			zap.String("period", period))
		return
	}

	// API 数据入库
	klineModels := make([]*models.Kline, 0, len(apiKlines))
	for _, k := range apiKlines {
		klineModels = append(klineModels, convertToModel(int(symbolID), period, k))
	}

	if err := s.klineRepo.BatchCreate(klineModels); err != nil {
		s.logger.Warn("后台同步：数据入库失败",
			zap.String("symbol", symbol.SymbolCode),
			zap.Int("count", len(apiKlines)),
			zap.Error(err))
		return
	}

	s.logger.Info("后台同步：K线数据同步完成",
		zap.String("symbol", symbol.SymbolCode),
		zap.String("period", period),
		zap.Int("synced_count", len(apiKlines)))
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

// needSupplement 判断是否需要从API补充数据，返回是否需要+起止时间
func (s *KlineService) needSupplement(klines []models.Kline, period string, startTime, endTime *time.Time, limit int) (bool, time.Time, time.Time) {
	periodDuration := PeriodToDuration[period]
	if periodDuration == 0 {
		periodDuration = time.Hour
	}

	if startTime != nil && endTime != nil {
		// 有明确时间范围：实际数量不足期望的 70%
		expectedCount := int(endTime.Sub(*startTime) / periodDuration)
		if expectedCount > 0 && len(klines) < int(float64(expectedCount)*0.7) {
			return true, *startTime, *endTime
		}
	} else if len(klines) < limit && limit > 0 {
		// 无时间范围但数量不足请求的 limit
		return true, time.Now().UTC().Add(-time.Duration(limit) * periodDuration), time.Now().UTC()
	}

	return false, time.Time{}, time.Time{}
}
