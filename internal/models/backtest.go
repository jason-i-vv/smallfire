package models

import "time"

// BacktestRequest 回测请求
type BacktestRequest struct {
	SymbolCode     string  `json:"symbol_code" binding:"required"`     // 标的代码
	MarketCode     string  `json:"market_code" binding:"required"`    // 市场代码
	Period         string  `json:"period" binding:"required"`          // 周期: 15m, 1h, 1d
	StrategyType   string  `json:"strategy_type" binding:"required"`    // 策略类型: box, trend, key_level, volume_price
	StartTime      string  `json:"start_time" binding:"required"`      // 开始时间 (UTC+8): 2024-01-01 00:00:00
	EndTime        string  `json:"end_time" binding:"required"`        // 结束时间 (UTC+8): 2024-12-31 23:59:59
	InitialCapital float64 `json:"initial_capital"`                    // 初始资金，默认 100000
	PositionSize   float64 `json:"position_size"`                    // 单笔仓位比例，默认 0.1
	StopLossPct    float64 `json:"stop_loss_pct"`                    // 止损比例，默认 0.02
	TakeProfitPct  float64 `json:"take_profit_pct"`                  // 止盈比例，默认 0.05
	EnableTrade    bool    `json:"enable_trade"`                     // 是否执行交易，默认 false
}

// BacktestResponse 回测响应
type BacktestResponse struct {
	Request     *BacktestRequest  `json:"request"`      // 请求参数
	Statistics  *BacktestStats    `json:"statistics"`  // 统计数据
	Trades      []*BacktestTrade  `json:"trades"`       // 交易列表
	Signals     []*Signal         `json:"signals"`      // 信号列表
	EquityCurve []*EquityPoint    `json:"equity_curve"` // 权益曲线
	Boxes       []*Box            `json:"boxes"`       // 箱体列表（箱体策略）
	Trends      []*Trend          `json:"trends"`      // 趋势列表（趋势策略）
	RunTimeMs   int64             `json:"run_time_ms"` // 运行时间(毫秒)
}

// BacktestStats 回测统计
type BacktestStats struct {
	TotalTrades     int     `json:"total_trades"`      // 总交易次数
	WinTrades       int     `json:"win_trades"`        // 盈利次数
	LoseTrades      int     `json:"lose_trades"`       // 亏损次数
	WinRate         float64 `json:"win_rate"`           // 胜率
	TotalPnL        float64 `json:"total_pnl"`         // 总盈亏
	TotalPnLPercent float64 `json:"total_pnl_percent"` // 总盈亏比例
	AvgWin          float64 `json:"avg_win"`            // 平均盈利
	AvgLoss         float64 `json:"avg_loss"`           // 平均亏损
	ProfitFactor    float64 `json:"profit_factor"`      // 盈亏比
	Expectancy      float64 `json:"expectancy"`         // 期望值
	MaxDrawdown     float64 `json:"max_drawdown"`      // 最大回撤
	MaxDrawdownPct float64 `json:"max_drawdown_pct"`  // 最大回撤比例
	SharpeRatio     float64 `json:"sharpe_ratio"`      // 夏普比率
	FinalCapital    float64 `json:"final_capital"`     // 最终资金
}

// BacktestTrade 回测交易记录
type BacktestTrade struct {
	ID             int        `json:"id"`              // 交易ID
	SignalID       int        `json:"signal_id"`       // 信号ID
	EntryTime      time.Time  `json:"entry_time"`      // 入场时间
	ExitTime       *time.Time `json:"exit_time"`       // 出场时间
	Direction      string     `json:"direction"`      // 方向: long, short
	EntryPrice     float64    `json:"entry_price"`     // 入场价格
	ExitPrice      float64    `json:"exit_price"`      // 出场价格
	Quantity       float64    `json:"quantity"`         // 数量
	PnL            float64    `json:"pnl"`              // 盈亏
	PnLPercent     float64    `json:"pnl_percent"`     // 盈亏比例
	Fees           float64    `json:"fees"`             // 手续费
	ExitReason     string     `json:"exit_reason"`     // 出场原因: stop_loss, take_profit, trailing_stop
	HoldHours      float64    `json:"hold_hours"`      // 持仓时长(小时)
	CumPnL         float64    `json:"cum_pnl"`         // 累计盈亏
}

// EquityPoint 权益曲线数据点
type EquityPoint struct {
	Time   time.Time `json:"time"`   // 时间
	Capital float64  `json:"capital"` // 资金
	PnL     float64  `json:"pnl"`    // 盈亏
}

// BacktestStatus 回测状态
type BacktestStatus struct {
	Status    string `json:"status"`     // running, completed, failed
	Progress  int    `json:"progress"`   // 进度 0-100
	Message   string `json:"message"`    // 状态消息
	ResultURL string `json:"result_url"` // 结果URL
}
