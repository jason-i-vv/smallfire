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
	notifier      DependencyNotifier
	interval      time.Duration
	maxConcurrent int
	stopCh        chan struct{}
	wg            sync.WaitGroup
	logger        *zap.Logger
}

// DependencyNotifier 信号通知接口
type DependencyNotifier interface {
	SendSignal(signal *models.Signal) error
}

// NewRunner 创建策略运行器
func NewRunner(factory *Factory, klineRepo repository.KlineRepo,
	symbolRepo repository.SymbolRepo, signalRepo repository.SignalRepo,
	notifier DependencyNotifier,
	interval time.Duration, maxConcurrent int, logger *zap.Logger) *Runner {
	if maxConcurrent <= 0 {
		maxConcurrent = 20
	}
	return &Runner{
		factory:       factory,
		klineRepo:     klineRepo,
		symbolRepo:    symbolRepo,
		signalRepo:    signalRepo,
		notifier:      notifier,
		interval:      interval,
		maxConcurrent: maxConcurrent,
		logger:        logger,
	}
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

		// 运行所有策略
		for _, strategy := range r.factory.ListStrategies() {
			r.logger.Debug("运行策略",
				zap.String("strategy", strategy.Name()),
				zap.String("symbol", symbol.Code),
				zap.String("period", period),
				zap.Int("klines", len(klines)))
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

				if r.shouldCreateSignal(signal) {
					if err := r.signalRepo.Create(signal); err != nil {
						r.logger.Error("创建信号失败", zap.Error(err))
						continue
					}

					// 发送飞书通知
					if r.notifier != nil {
						if err := r.notifier.SendSignal(signal); err != nil {
							r.logger.Error("发送信号通知失败",
								zap.String("signal_type", signal.SignalType),
								zap.String("symbol", signal.SymbolCode),
								zap.Error(err))
						}
					}
				}
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
func (r *Runner) shouldCreateSignal(signal *models.Signal) bool {
	// 检查相同类型的信号是否在短时间内已存在
	existingSignals, err := r.signalRepo.GetBySymbol(signal.SymbolID)
	if err != nil {
		return true
	}

	// 检查最近1小时内是否有相同类型的信号
	cutoffTime := time.Now().Add(-1 * time.Hour)
	for _, existing := range existingSignals {
		if existing.SignalType == signal.SignalType &&
			existing.CreatedAt.After(cutoffTime) {
			return false
		}
	}

	return true
}
