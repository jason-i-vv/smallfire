package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/smallfire/starfire/internal/config"
	"github.com/smallfire/starfire/internal/database"
	"github.com/smallfire/starfire/internal/handler"
	"github.com/smallfire/starfire/internal/repository"
	"github.com/smallfire/starfire/internal/service/ema"
	"github.com/smallfire/starfire/internal/service/market"
	"github.com/smallfire/starfire/internal/service/strategy"
	"github.com/smallfire/starfire/internal/service/trading"
	"github.com/smallfire/starfire/pkg/utils"
)

func main() {
	// 加载配置
	configPath := "config/config.yml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Printf("配置加载失败: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志
	utils.InitLogger(cfg.Log)
	defer func() {
		if utils.Logger != nil {
			_ = utils.Logger.Sync()
		}
	}()
	utils.Info("日志系统初始化成功")

	// 初始化数据库连接
	db, err := database.NewPostgresDB(cfg.Database)
	if err != nil {
		utils.Fatal("数据库连接失败", zap.Error(err))
	}
	defer db.Close()
	utils.Info("数据库连接成功")

	// 初始化 Repository
	marketRepo := repository.NewMarketRepoPG(db)
	symbolRepo := repository.NewSymbolRepoPG(db)
	klineRepo := repository.NewKlineRepoPG(db)
	signalRepo := repository.NewSignalRepoPG(db)
	boxRepo := repository.NewBoxRepoPG(db)
	trendRepo := repository.NewTrendRepoPG(db)
	keyLevelRepo := repository.NewKeyLevelRepoPG(db)
	trackRepo := repository.NewTradeTrackRepoPG(db)

	// 初始化交易服务
	tradingDeps := trading.Dependency{
		TrackRepo:  trackRepo,
		SignalRepo: signalRepo,
		Logger:     utils.Logger,
	}
	tradeExecutor := trading.NewTradeExecutor(&cfg.Trading, tradingDeps)
	_ = tradeExecutor
	utils.Info("交易执行器初始化成功")

	// 初始化统计分析服务
	statsService := trading.NewStatisticsService(trackRepo, &cfg.Trading)
	_ = statsService
	utils.Info("统计分析服务初始化成功")

	// 初始化持仓监控服务
	positionMonitor := trading.NewPositionMonitor(tradeExecutor, trackRepo, symbolRepo, utils.Logger)
	positionMonitor.Start() // 启动持仓监控
	defer positionMonitor.Stop()

	// 初始化 EMA 计算器
	emaCalc := ema.NewEMACalculator(cfg.EMA.Periods)
	utils.Info("EMA计算器初始化成功")

	// 初始化行情抓取器工厂
	factory := market.NewFactory(&cfg.Markets, symbolRepo, klineRepo)
	utils.Info("行情抓取器工厂初始化成功", zap.Int("fetcher_count", factory.Count()))

	// 初始化 K线查询服务
	klineService := market.NewKlineService(klineRepo, factory, utils.Logger)
	_ = klineService
	utils.Info("K线查询服务初始化成功")

	// 初始化热度管理器
	hotManager := market.NewHotManager(symbolRepo, marketRepo, factory, &cfg.Markets, utils.Logger)
	utils.Info("热度管理器初始化成功")

	// 初始化策略依赖
	strategyDeps := strategy.Dependency{
		BoxRepo:     boxRepo,
		TrendRepo:   trendRepo,
		LevelRepo:   keyLevelRepo,
		SignalRepo:  signalRepo,
		KlineRepo:   klineRepo,
		Notifier:    nil, // 暂时不实现通知功能
	}

	// 初始化策略工厂
	strategyFactory := strategy.NewFactory(&cfg.Strategies, strategyDeps, utils.Logger)
	utils.Info("策略工厂初始化成功", zap.Int("strategy_count", len(strategyFactory.ListStrategies())))

	// 初始化策略运行器
	strategyRunner := strategy.NewRunner(strategyFactory, klineRepo, symbolRepo, signalRepo, 5*time.Minute, utils.Logger)
	utils.Info("策略运行器初始化成功")

	// 初始化同步服务
	syncService := market.NewSyncService(factory, klineRepo, symbolRepo, emaCalc, utils.Logger, &cfg.Markets)
	utils.Info("同步服务初始化成功")

	// 初始化并更新热度标的
	if err := hotManager.UpdateHotSymbols(); err != nil {
		utils.Error("初始化热度标的失败", zap.Error(err))
	} else {
		utils.Info("初始化热度标的成功")
	}

	// 启动策略运行器
	strategyRunner.Start()
	defer strategyRunner.Stop()

	// 启动同步服务
	syncService.Start()
	defer syncService.Stop()

	// 设置 Gin 模式
	if cfg.App.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	// 创建 Gin 实例
	r := gin.Default()

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "healthy",
			"service": cfg.App.Name,
			"version": "1.0.0",
			"time":    time.Now().Unix(),
		})
	})

	// 初始化 API 处理器
	marketHandler := handler.NewMarketHandler(marketRepo, utils.Logger)
	symbolHandler := handler.NewSymbolHandler(symbolRepo, klineRepo, utils.Logger)
	signalHandler := handler.NewSignalHandler(signalRepo, utils.Logger)
	strategyHandler := handler.NewStrategyHandler(&cfg.Strategies, utils.Logger)
	tradeHandler := handler.NewTradeHandler(trackRepo, statsService, utils.Logger)

	// API 版本
	apiV1 := r.Group("/api/v1")
	{
		apiV1.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"code":    0,
				"message": "success",
				"data": gin.H{
					"status": "ok",
				},
				"timestamp": time.Now().Unix(),
			})
		})

		// 市场 API
		marketsGroup := apiV1.Group("/markets")
		{
			marketsGroup.GET("", marketHandler.GetMarkets)
			marketsGroup.GET("/:market_code", marketHandler.GetMarket)
		}

		// 交易标的 API
		symbolsGroup := apiV1.Group("/symbols")
		{
			symbolsGroup.GET("", symbolHandler.GetSymbols)
			symbolsGroup.GET("/:id/klines", symbolHandler.GetSymbolKlines)
		}

		// 市场交易标的 API
		marketSymbolsGroup := apiV1.Group("/markets/:market_code/symbols")
		{
			marketSymbolsGroup.GET("", symbolHandler.GetMarketSymbols)
		}

		// 策略信号 API
		signalsGroup := apiV1.Group("/signals")
		{
			signalsGroup.GET("", signalHandler.GetSignals)
			signalsGroup.GET("/:id", signalHandler.GetSignal)
		}

		// 标的信号 API
		symbolSignalsGroup := apiV1.Group("/symbols/:id/signals")
		{
			symbolSignalsGroup.GET("", signalHandler.GetSymbolSignals)
		}

		// 策略配置 API
		strategiesGroup := apiV1.Group("/strategies")
		{
			strategiesGroup.GET("", strategyHandler.GetStrategies)
		}

		// 交易跟踪 API
		tradesGroup := apiV1.Group("/trades")
		{
			tradesGroup.GET("/positions", tradeHandler.GetOpenPositions)
			tradesGroup.GET("/history", tradeHandler.GetTradeHistory)
			tradesGroup.GET("/closed", tradeHandler.GetClosedPositions)
			tradesGroup.GET("/stats", tradeHandler.GetTradeStats)
			tradesGroup.GET("/signal-analysis", tradeHandler.GetSignalAnalysis)
			tradesGroup.GET("/:id", tradeHandler.GetTradeDetail)
			tradesGroup.POST("/:id/close", tradeHandler.ClosePosition)
		}
	}

	utils.Info("路由初始化完成")

	// 启动服务器
	addr := fmt.Sprintf("%s:%d", cfg.App.Host, cfg.App.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	go func() {
		utils.Info("服务器启动", zap.String("address", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			utils.Fatal("服务器启动失败", zap.Error(err))
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	utils.Info("正在关闭服务器...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		utils.Fatal("服务器强制关闭", zap.Error(err))
	}

	utils.Info("服务器已关闭")
}
