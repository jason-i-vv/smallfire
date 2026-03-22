package market

import (
	"sync"
	"time"

	"github.com/smallfire/starfire/internal/config"
	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/internal/repository"
	"github.com/smallfire/starfire/internal/service/ema"
	"go.uber.org/zap"
)

// SyncService 行情同步服务
type SyncService struct {
	factory    *Factory
	klineRepo  repository.KlineRepo
	symbolRepo repository.SymbolRepo
	emaCalc    *ema.EMACalculator
	interval   time.Duration
	stopCh     chan struct{}
	wg         sync.WaitGroup
	logger     *zap.Logger
	config     *config.MarketsConfig
}

// NewSyncService 创建同步服务
func NewSyncService(factory *Factory, klineRepo repository.KlineRepo,
	symbolRepo repository.SymbolRepo, emaCalc *ema.EMACalculator,
	logger *zap.Logger, cfg *config.MarketsConfig) *SyncService {
	return &SyncService{
		factory:    factory,
		klineRepo:  klineRepo,
		symbolRepo: symbolRepo,
		emaCalc:    emaCalc,
		interval:   60 * time.Second,
		logger:     logger,
		config:     cfg,
	}
}

func (s *SyncService) Start() {
	s.stopCh = make(chan struct{})

	// 启动每个市场的同步任务
	for _, fetcher := range s.factory.ListEnabledFetchers() {
		s.wg.Add(1)
		go s.runSyncLoop(fetcher)
	}

	// 启动热度更新任务（每小时执行一次）
	s.wg.Add(1)
	go s.runHotUpdateLoop()

	s.logger.Info("同步服务已启动")
}

func (s *SyncService) Stop() {
	close(s.stopCh)
	s.wg.Wait()
	s.logger.Info("同步服务已停止")
}

func (s *SyncService) runSyncLoop(fetcher Fetcher) {
	defer s.wg.Done()

	marketCode := fetcher.MarketCode()
	periods := fetcher.SupportedPeriods()

	// 限制periods为配置的周期
	configuredPeriods := s.getConfiguredPeriods(marketCode)
	if len(configuredPeriods) > 0 {
		periods = configuredPeriods
	}

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			// 获取需要同步的标的
			symbols, err := s.symbolRepo.GetTrackingByMarket(marketCode)
			if err != nil {
				s.logger.Error("获取跟踪标的失败", zap.String("market", marketCode), zap.Error(err))
				continue
			}

			for _, symbol := range symbols {
				for _, period := range periods {
					if err := s.syncSymbolKlines(symbol, fetcher, period); err != nil {
						s.logger.Error("同步K线失败",
							zap.String("market", marketCode),
							zap.String("symbol", symbol.SymbolCode),
							zap.String("period", period),
							zap.Error(err))
					}
				}
			}
		}
	}
}

func (s *SyncService) runHotUpdateLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.logger.Info("开始更新热度标的")
			// 这里调用热度管理更新方法
			// TODO: 实现热度更新逻辑
		}
	}
}

func (s *SyncService) syncSymbolKlines(symbol *models.Symbol, fetcher Fetcher, period string) error {
	// 获取最新K线
	klines, err := fetcher.FetchKlines(symbol.SymbolCode, MapPeriod(fetcher.MarketCode(), period), 100)
	if err != nil {
		return err
	}

	// 存储到数据库
	for _, k := range klines {
		// 检查是否已存在
		exists, err := s.klineRepo.Exists(int64(symbol.ID), period, k.OpenTime)
		if err != nil {
			return err
		}
		if exists {
			continue
		}

		// 转换为模型
		kline := convertToModel(symbol.ID, period, k)
		// 计算EMA
		// 需要获取该标的该周期的历史K线来计算EMA
		history, err := s.klineRepo.GetBySymbolPeriod(int64(symbol.ID), period, nil, nil, 200)
		if err != nil {
			return err
		}
		history = append(history, *kline)
		calculated := s.emaCalc.Calculate(history)
		if len(calculated) > 0 {
			last := calculated[len(calculated)-1]
			kline.EMAShort = last.EMAShort
			kline.EMAMedium = last.EMAMedium
			kline.EMALong = last.EMALong
		}

		if err := s.klineRepo.Create(kline); err != nil {
			s.logger.Error("创建K线记录失败", zap.Error(err))
		}
	}

	return nil
}

func (s *SyncService) getConfiguredPeriods(marketCode string) []string {
	switch marketCode {
	case "bybit":
		return s.config.Bybit.Periods
	case "a_stock":
		return s.config.AStock.Periods
	case "us_stock":
		return s.config.USStock.Periods
	}
	return []string{}
}
