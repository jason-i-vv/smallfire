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
	factory    *Factory
	klineRepo  repository.KlineRepo
	symbolRepo repository.SymbolRepo
	signalRepo repository.SignalRepo
	interval   time.Duration
	stopCh     chan struct{}
	wg         sync.WaitGroup
	logger     *zap.Logger
}

// NewRunner 创建策略运行器
func NewRunner(factory *Factory, klineRepo repository.KlineRepo,
	symbolRepo repository.SymbolRepo, signalRepo repository.SignalRepo,
	interval time.Duration, logger *zap.Logger) *Runner {
	return &Runner{
		factory:    factory,
		klineRepo:  klineRepo,
		symbolRepo: symbolRepo,
		signalRepo: signalRepo,
		interval:   interval,
		logger:     logger,
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

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	// 首次立即执行一次
	r.analyzeAllSymbols()

	for {
		select {
		case <-r.stopCh:
			return
		case <-ticker.C:
			r.analyzeAllSymbols()
		}
	}
}

func (r *Runner) analyzeAllSymbols() {
	r.logger.Info("策略执行器开始分析所有标的")

	// 获取所有跟踪的标的
	symbols, err := r.klineRepo.GetAllTrackedSymbols()
	if err != nil {
		r.logger.Error("获取跟踪标的失败", zap.Error(err))
		return
	}

	for _, symbol := range symbols {
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

			//r.logger.Debug("策略执行器监听到新K线",
			//	zap.String("symbol", symbol.Code),
			//	zap.String("period", period),
			//	zap.Int("kline_count", len(klines)))

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

						// 发送通知 (TODO)
						// r.sendNotification(signal)
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

// sendNotification 发送通知
func (r *Runner) sendNotification(signal *models.Signal) {
	// TODO: 实现通知发送
	r.logger.Info("发送信号通知",
		zap.String("signal_type", signal.SignalType),
		zap.Int("symbol_id", signal.SymbolID))
}
