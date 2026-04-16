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
	"github.com/smallfire/starfire/internal/middleware"
	"github.com/smallfire/starfire/internal/repository"
	authservice "github.com/smallfire/starfire/internal/service/auth"
	"github.com/smallfire/starfire/internal/service/ema"
	"github.com/smallfire/starfire/internal/service/market"
	"github.com/smallfire/starfire/internal/service/monitoring"
	"github.com/smallfire/starfire/internal/service/notification"
	"github.com/smallfire/starfire/internal/service/backtest"
	aiservice "github.com/smallfire/starfire/internal/service/ai"
	"github.com/smallfire/starfire/internal/service/scoring"
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
	keyLevelV2Repo := repository.NewKeyLevelV2RepoPG(db)
	trackRepo := repository.NewTradeTrackRepoPG(db)
	monitorRepo := repository.NewMonitorRepoPG(db)
	notifyRepo := repository.NewNotificationRepoPG(db)
	userRepo := repository.NewUserRepoPG(db)

	// 初始化评分与交易机会相关 Repository
	oppRepo := repository.NewOpportunityRepoPG(db)
	statsRepo := repository.NewSignalTypeStatsRepoPG(db)

	// 初始化认证服务
	if cfg.JWT.Secret == "" {
		utils.Fatal("JWT密钥未配置，请设置 JWT_SECRET 环境变量")
	}
		authsvc := authservice.NewAuthService(userRepo, cfg.JWT.Secret, cfg.JWT.ExpirationDuration(), utils.Logger)
	utils.Info("认证服务初始化成功")

	// 初始化监测服务
	tickerRepo := repository.NewMemoryTickerRepo()
	monitorFactory := monitoring.NewFactory(tickerRepo, cfg.Monitoring.MaxConcurrentMonitors)
	monitorService := monitoring.NewService(
		monitorFactory,
		tickerRepo,
		monitorRepo,
		time.Duration(cfg.Monitoring.PriceCheckInterval)*time.Second,
	)
	monitorService.Start()
	defer monitorService.Stop()
	utils.Info("监测服务初始化成功")

	// 初始化交易服务
	tradingDeps := trading.Dependency{
		TrackRepo:  trackRepo,
		SignalRepo: signalRepo,
		OppRepo:    oppRepo,
		StatsRepo:  statsRepo,
		Logger:     utils.Logger,
	}
	tradeExecutor := trading.NewTradeExecutor(&cfg.Trading, tradingDeps)
	utils.Info("交易执行器初始化成功")

	// 初始化统计分析服务
	statsService := trading.NewStatisticsService(trackRepo, signalRepo, symbolRepo, &cfg.Trading)
	utils.Info("统计分析服务初始化成功")

	// 初始化通知服务
	feishuNotifier := notification.NewFeishuNotifier(&notification.FeishuConfig{
		Enabled:         cfg.Feishu.Enabled,
		WebhookURL:      cfg.Feishu.WebhookURL,
		SendSummary:     cfg.Feishu.SendSummary,
		SummaryInterval: cfg.Feishu.SummaryInterval,
		SummaryTimes:    cfg.Feishu.SummaryTimes,
	}, notifyRepo)

	// 初始化汇总服务
	summarySvc := notification.NewSummaryService(feishuNotifier, statsService, &notification.FeishuConfig{
		Enabled:         cfg.Feishu.Enabled,
		WebhookURL:      cfg.Feishu.WebhookURL,
		SendSummary:     cfg.Feishu.SendSummary,
		SummaryInterval: cfg.Feishu.SummaryInterval,
		SummaryTimes:    cfg.Feishu.SummaryTimes,
	})
	if cfg.Feishu.SendSummary {
		summarySvc.Start()
		defer summarySvc.Stop()
	}

	// 初始化通知管理器
	notifiers := []notification.Notifier{feishuNotifier}
	notifyManager := notification.NewManager(notifiers, summarySvc, notifyRepo)
	utils.Info("通知服务初始化成功")

	// 初始化持仓监控服务
	positionMonitor := trading.NewPositionMonitor(tradeExecutor, trackRepo, symbolRepo, utils.Logger)

	// 注入价格提供者（基于 K 线最新收盘价）
	priceProvider := trading.NewKlinePriceProvider(klineRepo)
	positionMonitor.SetPriceProvider(priceProvider)

	positionMonitor.Start() // 启动持仓监控
	defer positionMonitor.Stop()
	utils.Info("持仓监控服务初始化成功（价格源: K线）")

	// 初始化 EMA 计算器
	emaCalc := ema.NewEMACalculator(cfg.EMA.Periods)
	utils.Info("EMA计算器初始化成功")

	// 初始化行情抓取器工厂
	factory := market.NewFactory(&cfg.Markets, symbolRepo, klineRepo)
	utils.Info("行情抓取器工厂初始化成功", zap.Int("fetcher_count", factory.Count()))

	// 初始化 K线查询服务
	klineService := market.NewKlineService(klineRepo, symbolRepo, factory, utils.Logger)
	utils.Info("K线查询服务初始化成功")

	// 初始化热度管理器
	hotManager := market.NewHotManager(symbolRepo, marketRepo, factory, &cfg.Markets, utils.Logger)
	utils.Info("热度管理器初始化成功")

	// 初始化策略依赖
	strategyDeps := strategy.Dependency{
		BoxRepo:     boxRepo,
		TrendRepo:   trendRepo,
		LevelRepo:   keyLevelRepo,
		LevelV2Repo: keyLevelV2Repo,
		SignalRepo:  signalRepo,
		KlineRepo:   klineRepo,
		Notifier:    notifyManager,
	}

	// 初始化策略工厂（实盘：仅注册 enabled=true 的策略）
	strategyFactory := strategy.NewFactory(&cfg.Strategies, strategyDeps, utils.Logger, false)
	utils.Info("策略工厂初始化成功", zap.Int("strategy_count", len(strategyFactory.ListStrategies())))

	// 初始化策略运行器
	runnerInterval := 5 * time.Minute
	if cfg.Strategies.RunnerInterval > 0 {
		runnerInterval = time.Duration(cfg.Strategies.RunnerInterval) * time.Second
	}
	strategyRunner := strategy.NewRunner(strategyFactory, klineRepo, symbolRepo, signalRepo, runnerInterval, cfg.Strategies.MaxConcurrentAnalysis, utils.Logger)
	utils.Info("策略运行器初始化成功")

	// 初始化评分引擎
	signalScorer := scoring.NewSignalScorer(scoring.DefaultWeights)
	utils.Info("评分引擎初始化成功")

	// 初始化交易机会聚合器
	oppAggregator := scoring.NewOpportunityAggregator(oppRepo, signalRepo, statsRepo, signalScorer, scoring.DefaultValidityConfig, notifyManager, utils.Logger, cfg.Trading.MinNotifyScoreThreshold)
	strategyRunner.SetAggregator(oppAggregator)
	utils.Info("交易机会聚合器初始化成功")

	// 初始化自动交易服务（评分达标自动开仓）
	autoTrader := trading.NewAutoTrader(&cfg.Trading, trackRepo, signalRepo, klineRepo, utils.Logger)
	oppAggregator.AddHandler(autoTrader)
	utils.Info("自动交易服务初始化成功",
		zap.Bool("enabled", cfg.Trading.AutoTradeEnabled),
		zap.Int("score_threshold", cfg.Trading.AutoTradeScoreThreshold))

	// 初始化 AI 分析服务
	var aiClient *aiservice.AIClient
	var aiAnalyzer *aiservice.OpportunityAnalyzer
	var cooldownTracker *aiservice.CooldownTracker
	if cfg.AI.Enabled && cfg.AI.APIKey != "" {
		aiClient = aiservice.NewAIClient(cfg.AI)
		cooldownTracker = aiservice.NewCooldownTracker(
			cfg.AI.Judge.MaxDailyCalls,
			cfg.AI.Judge.CooldownMinutes,
		)
		aiAnalyzer = aiservice.NewOpportunityAnalyzer(
			aiClient, oppRepo, klineRepo, cfg.AI.Judge, cooldownTracker, utils.Logger,
		)
		utils.Info("AI 分析服务初始化成功",
			zap.String("model", cfg.AI.Model),
			zap.String("base_url", cfg.AI.BaseURL),
			zap.Bool("auto_analyze", cfg.AI.Judge.AutoAnalyze),
		)
		if cfg.AI.Judge.AutoAnalyze {
			oppAggregator.AddHandler(aiAnalyzer)
		}

		// 初始化 AI 关键价位识别器
		if cfg.AI.KeyLevel.Enabled {
			aiKeyLevelCooldown := aiservice.NewCooldownTracker(
				cfg.AI.KeyLevel.MaxDailyCalls,
				cfg.AI.KeyLevel.CooldownMinutes,
			)
			aiKeyLevelAnalyzer := aiservice.NewAIKeyLevelAnalyzer(
				aiClient, klineRepo, keyLevelV2Repo, klineRepo,
				cfg.AI.KeyLevel, aiKeyLevelCooldown, utils.Logger,
			)
			go aiKeyLevelAnalyzer.Run()
			defer aiKeyLevelAnalyzer.Stop()
			utils.Info("AI关键价位识别器启动",
				zap.Int("interval_minutes", cfg.AI.KeyLevel.IntervalMinutes))
		}
	} else {
		utils.Info("AI 分析服务未启用")
	}

	// 初始化回测策略工厂（回测：注册所有策略，方便测试）
	backtestStrategyFactory := strategy.NewFactory(&cfg.Strategies, strategyDeps, utils.Logger, true)

	// 初始化回测服务
	backtestService := backtest.NewBacktestService(klineRepo, symbolRepo, backtestStrategyFactory, factory, emaCalc, *cfg, utils.Logger)
	utils.Info("回测服务初始化成功")

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

	// 启动同步服务（K线同步后立即触发策略分析）
	syncService.AddHook(strategyRunner)
	syncService.Start()
	defer syncService.Stop()

	// 设置 Gin 模式
	if cfg.App.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	// 创建 Gin 实例
	r := gin.Default()

	// CORS 中间件
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")
		c.Header("Access-Control-Expose-Headers", "Content-Length, Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

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
	marketHandler := handler.NewMarketHandler(marketRepo, symbolRepo, klineRepo, trendRepo, utils.Logger)
	symbolHandler := handler.NewSymbolHandler(symbolRepo, klineRepo, klineService, utils.Logger)
	signalHandler := handler.NewSignalHandler(signalRepo, utils.Logger)
	opportunityHandler := handler.NewOpportunityHandler(oppRepo, trackRepo, signalScorer, aiAnalyzer, cfg.AI, cooldownTracker, utils.Logger)
	strategyHandler := handler.NewStrategyHandler(&cfg.Strategies, utils.Logger)
	tradeHandler := handler.NewTradeHandler(trackRepo, tradeExecutor, statsService, utils.Logger)
	backtestHandler := handler.NewBacktestHandler(backtestService, utils.Logger)
	boxHandler := handler.NewBoxHandler(boxRepo, symbolRepo, utils.Logger)
	keyLevelHandler := handler.NewKeyLevelHandler(keyLevelV2Repo, utils.Logger)
	trendHandler := handler.NewTrendHandler(trendRepo, utils.Logger)
	aiStatsSvc := aiservice.NewAIStatsService(db)
	aiStatsHandler := handler.NewAIStatsHandler(aiStatsSvc, utils.Logger)
authHandler := handler.NewAuthHandler(authsvc, utils.Logger)
	userHandler := handler.NewUserHandler(authsvc, utils.Logger)

	// API 版本
	apiV1 := r.Group("/api/v1")
	{
		apiV1.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{
				"code":    0,
				"message": "success",
				"data": gin.H{
					"status":       "ok",
					"auth_enabled": cfg.JWT.Enabled,
				},
					"timestamp": time.Now().Unix(),
			})
		})

		// 公开路由（无需认证）
		authGroup := apiV1.Group("/auth")
		{
			authGroup.POST("/register", authHandler.Register)
			authGroup.POST("/login", authHandler.Login)
		}

		// 需要认证的路由
	authenticated := apiV1.Group("")
			if cfg.JWT.Enabled {
				authenticated.Use(middleware.AuthMiddleware(cfg.JWT.Secret, userRepo))
			}
		{
			// 认证相关
			authenticated.GET("/auth/me", authHandler.Me)
			authenticated.PUT("/auth/password", authHandler.ChangePassword)

			// 市场 API
			marketsGroup := authenticated.Group("/markets")
			{
				marketsGroup.GET("", marketHandler.GetMarkets)
				marketsGroup.GET("/:market_code", marketHandler.GetMarket)
				marketsGroup.GET("/:market_code/overview", marketHandler.GetMarketOverview)
			}

			// 交易标的 API
			symbolsGroup := authenticated.Group("/symbols")
			{
				symbolsGroup.GET("", symbolHandler.GetSymbols)
				symbolsGroup.GET("/resolve", symbolHandler.ResolveSymbol)
				symbolsGroup.GET("/:id/klines", symbolHandler.GetSymbolKlines)
			}

			// K线数据 API
			authenticated.GET("/klines", symbolHandler.GetKlines)

			// 市场交易标的 API
			marketSymbolsGroup := authenticated.Group("/markets/:market_code/symbols")
			{
				marketSymbolsGroup.GET("", symbolHandler.GetMarketSymbols)
			}

			// 策略信号 API
			signalsGroup := authenticated.Group("/signals")
			{
				signalsGroup.GET("", signalHandler.GetSignals)
				signalsGroup.GET("/counts", signalHandler.GetSignalCounts)
				signalsGroup.GET("/:id", signalHandler.GetSignal)
			}

			// 标的信号 API
			symbolSignalsGroup := authenticated.Group("/symbols/:id/signals")
			{
				symbolSignalsGroup.GET("", signalHandler.GetSymbolSignals)
			}

			// 交易机会 API
			opportunitiesGroup := authenticated.Group("/opportunities")
			{
				opportunitiesGroup.GET("", opportunityHandler.GetOpportunities)
				opportunitiesGroup.GET("/active", opportunityHandler.GetActiveOpportunities)
				opportunitiesGroup.GET("/:id", opportunityHandler.GetOpportunity)
				opportunitiesGroup.GET("/:id/trades", opportunityHandler.GetOpportunityTrades)
				opportunitiesGroup.POST("/:id/ai-analysis", opportunityHandler.AIAnalysis)
			}

			// 策略配置 API
			strategiesGroup := authenticated.Group("/strategies")
			{
				strategiesGroup.GET("", strategyHandler.GetStrategies)
			}

			// 箱体 API
			boxesGroup := authenticated.Group("/boxes")
			{
				boxesGroup.GET("", boxHandler.GetBoxes)
				boxesGroup.GET("/:id", boxHandler.GetBox)
			}

			// 标的箱体 API
			symbolBoxesGroup := authenticated.Group("/symbols/:id/boxes")
			{
				symbolBoxesGroup.GET("", boxHandler.GetBoxesBySymbol)
			}

			// 关键价位 API
			keyLevelsGroup := authenticated.Group("/key-levels")
			{
				keyLevelsGroup.GET("", keyLevelHandler.GetAllKeyLevels)
			}

			// 标的的关键价位 API
			symbolKeyLevelsGroup := authenticated.Group("/symbols/:id/key-levels")
			{
				symbolKeyLevelsGroup.GET("", keyLevelHandler.GetKeyLevelsBySymbol)
			}

				// 趋势 API
				symbolTrendsGroup := authenticated.Group("/symbols/:id/trends")
				{
					symbolTrendsGroup.GET("", trendHandler.GetTrendsBySymbol)
				}

			// 回测 API
			backtestGroup := authenticated.Group("/backtest")
			{
				backtestGroup.POST("", backtestHandler.RunBacktest)
				backtestGroup.GET("/strategies", backtestHandler.GetSupportedStrategies)
				backtestGroup.GET("/periods", backtestHandler.GetSupportedPeriods)
			}

			// AI 统计分析 API
			aiStatsGroup := authenticated.Group("/ai-stats")
			{
				aiStatsGroup.GET("/daily", aiStatsHandler.GetDailyCallStats)
				aiStatsGroup.GET("/overview", aiStatsHandler.GetOverview)
				aiStatsGroup.GET("/accuracy", aiStatsHandler.GetAccuracyAnalysis)
				aiStatsGroup.GET("/direction", aiStatsHandler.GetDirectionStats)
				aiStatsGroup.GET("/confidence", aiStatsHandler.GetConfidenceAnalysis)
			}

			// 交易跟踪 API
			tradesGroup := authenticated.Group("/trades")
			{
				tradesGroup.GET("/positions", tradeHandler.GetOpenPositions)
				tradesGroup.GET("/history", tradeHandler.GetTradeHistory)
				tradesGroup.GET("/closed", tradeHandler.GetClosedPositions)
				tradesGroup.GET("/stats", tradeHandler.GetTradeStats)
				tradesGroup.GET("/signal-analysis", tradeHandler.GetSignalAnalysis)
				tradesGroup.GET("/equity-curve", tradeHandler.GetEquityCurve)
				tradesGroup.GET("/symbol-analysis", tradeHandler.GetSymbolAnalysis)
				tradesGroup.GET("/direction-analysis", tradeHandler.GetDirectionAnalysis)
				tradesGroup.GET("/exit-reason-analysis", tradeHandler.GetExitReasonAnalysis)
				tradesGroup.GET("/period-pnl", tradeHandler.GetPeriodPnL)
				tradesGroup.GET("/pnl-distribution", tradeHandler.GetPnLDistribution)
				tradesGroup.GET("/signal-analysis-detail", tradeHandler.GetDetailedSignalAnalysis)
				tradesGroup.GET("/:id", tradeHandler.GetTradeDetail)
				tradesGroup.POST("/:id/close", tradeHandler.ClosePosition)
			}

			// 管理员 API
			adminGroup := authenticated.Group("/users")
			adminGroup.Use(middleware.RequireRole("admin"))
			{
				adminGroup.GET("", userHandler.ListUsers)
				adminGroup.PUT("/:id/status", userHandler.UpdateUserStatus)
				adminGroup.PUT("/:id/password", userHandler.ResetPassword)
			}
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

	// 先关闭HTTP服务器
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := srv.Shutdown(ctx); err != nil {
		utils.Warn("HTTP服务器强制关闭", zap.Error(err))
	}
	cancel()

	// 后台服务关闭（每个服务有自己的清理逻辑，可能需要更长时间）
	// 注意：defer 会按注册顺序的反序执行
	utils.Info("等待后台服务关闭...")
	time.Sleep(1 * time.Second) // 给一点时间让 defer 开始执行

	utils.Info("服务器已关闭")
}
