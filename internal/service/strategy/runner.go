package strategy

import (
	"sync"
	"time"

	"github.com/smallfire/starfire/internal/models"
	"github.com/smallfire/starfire/internal/repository"
	"go.uber.org/zap"
)

// Runner 策略运行器
type Runner struct {
	factory       *Factory
	klineRepo     repository.KlineRepo
	symbolRepo    repository.SymbolRepo
	signalRepo    repository.SignalRepo
	aggregator    OpportunityAggregator
	interval      time.Duration
	maxConcurrent int
	stopCh        chan struct{}
	wg            sync.WaitGroup
	logger        *zap.Logger
}

// OpportunityAggregator 交易机会聚合接口
type OpportunityAggregator interface {
	AggregateSignals(signals []*models.Signal) error
}

// NewRunner 创建策略运行器
func NewRunner(factory *Factory, klineRepo repository.KlineRepo,
	symbolRepo repository.SymbolRepo, signalRepo repository.SignalRepo,
	interval time.Duration, maxConcurrent int, logger *zap.Logger) *Runner {
	if maxConcurrent <= 0 {
		maxConcurrent = 20
	}
	return &Runner{
		factory:       factory,
		klineRepo:     klineRepo,
		symbolRepo:    symbolRepo,
		signalRepo:    signalRepo,
		aggregator:    nil, // 聚合器可选，通过 SetAggregator 设置
		interval:      interval,
		maxConcurrent: maxConcurrent,
		logger:        logger,
	}
}

// SetAggregator 设置交易机会聚合器
func (r *Runner) SetAggregator(aggregator OpportunityAggregator) {
	r.aggregator = aggregator
}

func (r *Runner) Start() {
	r.stopCh = make(chan struct{})

	// 启动策略分析循环
	r.wg.Add(1)
	go r.runAnalysisLoop()

	r.logger.Info("策略执行器已启动", zap.Duration("check_interval", r.interval))
}

func (r *Runner) Stop() {
	close(r.stopCh)

	// 等待最多10秒让当前任务完成
	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		r.logger.Info("策略运行器已停止")
	case <-time.After(10 * time.Second):
		r.logger.Warn("策略运行器停止超时，强制退出")
	}
}

func (r *Runner) runAnalysisLoop() {
	defer r.wg.Done()

	// 首次立即执行一次
	r.analyzeAllSymbols()

	for {
		// 计算下一个整点对齐时间
		nextRun := r.nextAlignedTime()
		waitDuration := time.Until(nextRun)

		r.logger.Debug("下次策略执行时间",
			zap.Time("next_run", nextRun),
			zap.Duration("wait", waitDuration))

		select {
		case <-r.stopCh:
			return
		case <-time.After(waitDuration):
			r.analyzeAllSymbols()
		}
	}
}

// nextAlignedTime 计算下一个整点对齐时间
// 例如 5 分钟间隔 → :00, :05, :10, :15 ...
func (r *Runner) nextAlignedTime() time.Time {
	now := time.Now()
	intervalMinutes := int(r.interval.Minutes())
	minutes := now.Minute()
	aligned := (minutes / intervalMinutes) * intervalMinutes
	next := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), aligned, 0, 0, now.Location())
	if !next.After(now) {
		next = next.Add(r.interval)
	}
	return next
}

func (r *Runner) analyzeAllSymbols() {
	analyzeStart := time.Now()
	r.logger.Info("策略执行器开始分析所有标的")

	// 获取所有跟踪的标的
	symbols, err := r.klineRepo.GetAllTrackedSymbols()
	if err != nil {
		r.logger.Error("获取跟踪标的失败", zap.Error(err))
		return
	}

	// 使用 semaphore 控制并发数
	sem := make(chan struct{}, r.maxConcurrent)
	var wg sync.WaitGroup

	for _, symbol := range symbols {
		wg.Add(1)
		go func(sym *repository.TrackedSymbol) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			r.analyzeSymbol(sym)
		}(symbol)
	}

	wg.Wait()

	r.logger.Info("策略执行器完成分析",
		zap.Int("symbol_count", len(symbols)),
		zap.Duration("duration", time.Since(analyzeStart)))
}

// analyzeSymbol 分析单个标的所有周期和策略
// AnalyzeSymbol 分析单个标的（导出，供外部钩子调用）
func (r *Runner) AnalyzeSymbol(symbolID int, symbolCode, marketCode string) {
	symbol := &repository.TrackedSymbol{
		ID:         symbolID,
		Code:       symbolCode,
		MarketCode: marketCode,
	}
	r.analyzeSymbol(symbol)
}

// OnKlinesSynced 实现 market.SyncHook 接口，K 线同步完成后立即触发策略分析
func (r *Runner) OnKlinesSynced(symbolID int, symbolCode, marketCode, period string) {
	go r.AnalyzeSymbol(symbolID, symbolCode, marketCode)
}

func (r *Runner) analyzeSymbol(symbol *repository.TrackedSymbol) {
	for _, period := range r.getSymbolPeriods(symbol.MarketCode) {
		// 获取最新K线
		klines, err := r.klineRepo.GetLatestN(symbol.ID, period, 200)
		if err != nil {
			r.logger.Error("获取K线失败",
				zap.String("symbol", symbol.Code),
				zap.String("period", period),
				zap.Error(err))
			continue
		}
		if len(klines) < 10 {
			continue
		}

		// GetLatestN 返回倒序，需要反转为正序（旧->新）
		for i, j := 0, len(klines)-1; i < j; i, j = i+1, j-1 {
			klines[i], klines[j] = klines[j], klines[i]
		}

		// 过滤掉未收盘的 K 线，未收盘的 K 线不应该参与策略分析
		now := time.Now()
		closedCount := 0
		for _, k := range klines {
			if k.CloseTime.After(now) {
				continue
			}
			closedCount++
		}
		if closedCount != len(klines) {
			filtered := make([]models.Kline, 0, closedCount)
			for _, k := range klines {
				if !k.CloseTime.After(now) {
					filtered = append(filtered, k)
				}
			}
			klines = filtered
			if len(klines) < 10 {
				continue
			}
		}

		// 运行所有策略，收集所有新信号
		var allNewSignals []*models.Signal
		for _, strategy := range r.factory.ListStrategies() {
			signals, err := strategy.Analyze(symbol.ID, symbol.Code, period, klines)
			if err != nil {
				r.logger.Error("策略分析失败",
					zap.String("strategy", strategy.Name()),
					zap.String("symbol", symbol.Code),
					zap.Error(err))
				continue
			}

			// 保存信号
			for i := range signals {
				signal := &signals[i]
				signal.SymbolID = symbol.ID
				signal.SymbolCode = symbol.Code

				if r.shouldCreateSignal(signal) {
					if err := r.signalRepo.Create(signal); err != nil {
						r.logger.Error("创建信号失败", zap.Error(err))
						continue
					}

					allNewSignals = append(allNewSignals, signal)
				}
			}
		}

		// 聚合信号到交易机会
		if r.aggregator != nil && len(allNewSignals) > 0 {
			if err := r.aggregator.AggregateSignals(allNewSignals); err != nil {
				r.logger.Error("聚合交易机会失败",
					zap.String("symbol", symbol.Code),
					zap.Error(err))
			}
		}
	}
}

// getSymbolPeriods 获取标的配置的周期
func (r *Runner) getSymbolPeriods(marketCode string) []string {
	// 根据市场配置返回对应的周期
	switch marketCode {
	case "bybit":
		return []string{"15m", "1h"}
	case "a_stock":
		return []string{"1d"}
	case "us_stock":
		return []string{"1d"}
	default:
		return []string{"15m", "1h"}
	}
}

// shouldCreateSignal 判断是否应该创建信号（去重）
// 基于 K 线时间去重：同一 symbol + period + signal_type + kline_time 已存在则跳过
func (r *Runner) shouldCreateSignal(signal *models.Signal) bool {
	exists, err := r.signalRepo.ExistsDuplicate(signal.SymbolID, signal.SignalType, signal.Period, signal.KlineTime)
	if err != nil {
		r.logger.Warn("查询重复信号失败", zap.Error(err))
		return true
	}
	return !exists
}
