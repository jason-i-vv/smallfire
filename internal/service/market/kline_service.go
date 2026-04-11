package market

import (
	"fmt"
	"sort"
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

// GetKlines 获取K线数据，DB数据不足时从交易所API补充
func (s *KlineService) GetKlines(symbolID int64, period string, startTime, endTime *time.Time, limit int) ([]models.Kline, error) {
	// 并发去重：防止同一标的并发触发多次 API 请求
	dedupKey := fmt.Sprintf("%d:%s", symbolID, period)
	s.sfMu.Lock()
	if _, inFlight := s.sfInFlight[dedupKey]; inFlight {
		s.sfMu.Unlock()
		// 其他请求正在补充，直接返回本地数据
		dbKlines, _ := s.klineRepo.GetBySymbolPeriod(symbolID, period, startTime, endTime, limit)
		return dbKlines, nil
	}
	s.sfInFlight[dedupKey] = struct{}{}
	s.sfMu.Unlock()
	defer func() {
		s.sfMu.Lock()
		delete(s.sfInFlight, dedupKey)
		s.sfMu.Unlock()
	}()

	// 1. 先查本地数据库
	dbKlines, err := s.klineRepo.GetBySymbolPeriod(symbolID, period, startTime, endTime, limit)
	if err != nil {
		return nil, err
	}

	// 2. 判断是否需要 API 补充
	needSupplement, fetchStart, fetchEnd := s.needSupplement(dbKlines, period, startTime, endTime, limit)
	if !needSupplement {
		return dbKlines, nil
	}

	// 3. 获取 symbol 信息
	symbol, err := s.symbolRepo.GetByID(int(symbolID))
	if err != nil {
		s.logger.Warn("获取标的信息失败，跳过API补充",
			zap.Int64("symbol_id", symbolID), zap.Error(err))
		return dbKlines, nil
	}

	// 4. 获取对应的 fetcher
	fetcher, ok := s.factory.GetFetcher(symbol.MarketCode)
	if !ok || fetcher == nil {
		s.logger.Warn("不支持的市场，跳过API补充",
			zap.String("market_code", symbol.MarketCode))
		return dbKlines, nil
	}

	// 5. 从交易所 API 获取数据
	apiPeriod := MapPeriod(symbol.MarketCode, period)
	apiKlines, err := fetcher.FetchKlinesByTimeRange(symbol.SymbolCode, apiPeriod, fetchStart, fetchEnd)
	if err != nil {
		s.logger.Warn("从交易所API获取K线数据失败，返回本地数据",
			zap.String("symbol", symbol.SymbolCode),
			zap.String("period", period),
			zap.Error(err))
		return dbKlines, nil
	}

	if len(apiKlines) == 0 {
		return dbKlines, nil
	}

	// 6. API 数据入库（ON CONFLICT DO NOTHING 幂等去重）
	klineModels := make([]*models.Kline, 0, len(apiKlines))
	for _, k := range apiKlines {
		klineModels = append(klineModels, convertToModel(int(symbolID), period, k))
	}

	if err := s.klineRepo.BatchCreate(klineModels); err != nil {
		s.logger.Warn("API数据入库失败",
			zap.String("symbol", symbol.SymbolCode),
			zap.Int("count", len(apiKlines)),
			zap.Error(err))
		// 入库失败仍尝试合并返回
		return s.mergeKlines(dbKlines, klineModels, limit), nil
	}

	// 7. 入库成功，重新从 DB 查询（unique constraint 保证去重）
	klines, err := s.klineRepo.GetBySymbolPeriod(symbolID, period, startTime, endTime, limit)
	if err != nil {
		s.logger.Warn("重新查询DB失败，使用合并数据",
			zap.Int64("symbol_id", symbolID), zap.Error(err))
		return s.mergeKlines(dbKlines, klineModels, limit), nil
	}

	s.logger.Info("API补充K线数据",
		zap.String("symbol", symbol.SymbolCode),
		zap.String("period", period),
		zap.Int("db_count", len(dbKlines)),
		zap.Int("api_count", len(apiKlines)),
		zap.Int("final_count", len(klines)))

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

// mergeKlines 合并DB和API的K线数据，按 open_time 去重（DB数据优先，可能有EMA）
func (s *KlineService) mergeKlines(dbKlines []models.Kline, apiKlines []*models.Kline, limit int) []models.Kline {
	klineMap := make(map[int64]models.Kline)

	// 先放 API 数据
	for _, k := range apiKlines {
		klineMap[k.OpenTime.Unix()] = *k
	}

	// 再放 DB 数据（覆盖 API 数据，因为 DB 数据可能有计算过的 EMA）
	for _, k := range dbKlines {
		klineMap[k.OpenTime.Unix()] = k
	}

	result := make([]models.Kline, 0, len(klineMap))
	for _, k := range klineMap {
		result = append(result, k)
	}

	// 按 open_time 升序排列
	s.sortKlines(result)

	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}

	return result
}

// sortKlines 按 open_time 升序排列
func (s *KlineService) sortKlines(klines []models.Kline) {
	sort.Slice(klines, func(i, j int) bool {
		return klines[i].OpenTime.Before(klines[j].OpenTime)
	})
}
