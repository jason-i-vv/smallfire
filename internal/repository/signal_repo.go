package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/smallfire/starfire/internal/database"
	"github.com/smallfire/starfire/internal/models"
)

// SignalRepoPG 信号数据访问实现
type SignalRepoPG struct {
	db *database.DB
}

// NewSignalRepoPG 创建信号数据访问实例
func NewSignalRepoPG(db *database.DB) SignalRepo {
	return &SignalRepoPG{
		db: db,
	}
}

func (r *SignalRepoPG) GetByID(id int) (*models.Signal, error) {
	var signal models.Signal
	query := `
		SELECT s.id, s.symbol_id, s.signal_type, s.source_type, s.direction, s.strength, s.price,
		       s.target_price, s.stop_loss_price, s.period, s.status, s.confirmed_at, s.expired_at,
		       s.triggered_at, s.notification_sent, s.kline_time, s.created_at,
		       COALESCE(sy.symbol_code, '') as symbol_code
		FROM signals s
		LEFT JOIN symbols sy ON s.symbol_id = sy.id
		WHERE s.id = $1
	`

	err := r.db.QueryRow(context.Background(), query, id).Scan(
		&signal.ID, &signal.SymbolID, &signal.SignalType, &signal.SourceType,
		&signal.Direction, &signal.Strength, &signal.Price, &signal.TargetPrice,
		&signal.StopLossPrice, &signal.Period, &signal.Status, &signal.ConfirmedAt,
		&signal.ExpiredAt, &signal.TriggeredAt, &signal.NotificationSent, &signal.KlineTime,
		&signal.CreatedAt, &signal.SymbolCode,
	)
	if err != nil {
		return nil, fmt.Errorf("查询信号详情失败: %w", err)
	}

	return &signal, nil
}

func (r *SignalRepoPG) GetActiveSignals() ([]*models.Signal, error) {
	var signals []*models.Signal
	query := `
		SELECT id, symbol_id, signal_type, source_type, direction, strength, price,
		       target_price, stop_loss_price, period, status, confirmed_at, expired_at,
		       triggered_at, notification_sent, created_at
		FROM signals
		WHERE status = $1 AND expired_at > $2
	`

	rows, err := r.db.Query(context.Background(), query, models.SignalStatusPending, time.Now())
	if err != nil {
		return nil, fmt.Errorf("查询活跃信号失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var signal models.Signal
		if err := rows.Scan(
			&signal.ID, &signal.SymbolID, &signal.SignalType, &signal.SourceType,
			&signal.Direction, &signal.Strength, &signal.Price, &signal.TargetPrice,
			&signal.StopLossPrice, &signal.Period, &signal.Status, &signal.ConfirmedAt,
			&signal.ExpiredAt, &signal.TriggeredAt, &signal.NotificationSent, &signal.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描信号数据失败: %w", err)
		}
		signals = append(signals, &signal)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历信号结果失败: %w", err)
	}

	return signals, nil
}

func (r *SignalRepoPG) GetByBatchID(batchID string) ([]*models.Signal, error) {
	var signals []*models.Signal
	query := `
		SELECT id, symbol_id, signal_type, source_type, direction, strength, price,
		       target_price, stop_loss_price, period, status, confirmed_at, expired_at,
		       triggered_at, notification_sent, created_at
		FROM signals
		WHERE signal_data->>'batch_id' = $1
	`

	rows, err := r.db.Query(context.Background(), query, batchID)
	if err != nil {
		return nil, fmt.Errorf("查询批次信号失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var signal models.Signal
		if err := rows.Scan(
			&signal.ID, &signal.SymbolID, &signal.SignalType, &signal.SourceType,
			&signal.Direction, &signal.Strength, &signal.Price, &signal.TargetPrice,
			&signal.StopLossPrice, &signal.Period, &signal.Status, &signal.ConfirmedAt,
			&signal.ExpiredAt, &signal.TriggeredAt, &signal.NotificationSent, &signal.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描批次信号数据失败: %w", err)
		}
		signals = append(signals, &signal)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历批次信号结果失败: %w", err)
	}

	return signals, nil
}

func (r *SignalRepoPG) GetByStatus(status string) ([]*models.Signal, error) {
	var signals []*models.Signal
	query := `
		SELECT id, symbol_id, signal_type, source_type, direction, strength, price,
		       target_price, stop_loss_price, period, status, confirmed_at, expired_at,
		       triggered_at, notification_sent, created_at
		FROM signals
		WHERE status = $1
	`

	rows, err := r.db.Query(context.Background(), query, status)
	if err != nil {
		return nil, fmt.Errorf("查询状态信号失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var signal models.Signal
		if err := rows.Scan(
			&signal.ID, &signal.SymbolID, &signal.SignalType, &signal.SourceType,
			&signal.Direction, &signal.Strength, &signal.Price, &signal.TargetPrice,
			&signal.StopLossPrice, &signal.Period, &signal.Status, &signal.ConfirmedAt,
			&signal.ExpiredAt, &signal.TriggeredAt, &signal.NotificationSent, &signal.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描状态信号数据失败: %w", err)
		}
		signals = append(signals, &signal)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历状态信号结果失败: %w", err)
	}

	return signals, nil
}

func (r *SignalRepoPG) GetByMarket(marketCode string) ([]*models.Signal, error) {
	var signals []*models.Signal
	query := `
		SELECT s.id, s.symbol_id, s.signal_type, s.source_type, s.direction, s.strength, s.price,
		       s.target_price, s.stop_loss_price, s.period, s.status, s.confirmed_at, s.expired_at,
		       s.triggered_at, s.notification_sent, s.created_at
		FROM signals s
		JOIN symbols sy ON s.symbol_id = sy.id
		JOIN markets m ON sy.market_id = m.id
		WHERE m.market_code = $1 AND s.expired_at > $2
	`

	rows, err := r.db.Query(context.Background(), query, marketCode, time.Now())
	if err != nil {
		return nil, fmt.Errorf("查询市场信号失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var signal models.Signal
		if err := rows.Scan(
			&signal.ID, &signal.SymbolID, &signal.SignalType, &signal.SourceType,
			&signal.Direction, &signal.Strength, &signal.Price, &signal.TargetPrice,
			&signal.StopLossPrice, &signal.Period, &signal.Status, &signal.ConfirmedAt,
			&signal.ExpiredAt, &signal.TriggeredAt, &signal.NotificationSent, &signal.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描市场信号数据失败: %w", err)
		}
		signals = append(signals, &signal)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历市场信号结果失败: %w", err)
	}

	return signals, nil
}

func (r *SignalRepoPG) GetBySymbol(symbolID int) ([]*models.Signal, error) {
	var signals []*models.Signal
	query := `
		SELECT id, symbol_id, signal_type, source_type, direction, strength, price,
		       target_price, stop_loss_price, period, status, confirmed_at, expired_at,
		       triggered_at, notification_sent, created_at
		FROM signals
		WHERE symbol_id = $1 AND expired_at > $2
	`

	rows, err := r.db.Query(context.Background(), query, symbolID, time.Now())
	if err != nil {
		return nil, fmt.Errorf("查询标的信号失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var signal models.Signal
		if err := rows.Scan(
			&signal.ID, &signal.SymbolID, &signal.SignalType, &signal.SourceType,
			&signal.Direction, &signal.Strength, &signal.Price, &signal.TargetPrice,
			&signal.StopLossPrice, &signal.Period, &signal.Status, &signal.ConfirmedAt,
			&signal.ExpiredAt, &signal.TriggeredAt, &signal.NotificationSent, &signal.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描标的信号数据失败: %w", err)
		}
		signals = append(signals, &signal)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历标的信号结果失败: %w", err)
	}

	return signals, nil
}

func (r *SignalRepoPG) Create(signal *models.Signal) error {
	query := `
		INSERT INTO signals (symbol_id, signal_type, source_type, direction, strength, price,
		                    target_price, stop_loss_price, period, status, confirmed_at, expired_at,
		                    triggered_at, notification_sent, kline_time, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, NOW())
		RETURNING id
	`

	err := r.db.QueryRow(context.Background(), query,
		signal.SymbolID, signal.SignalType, signal.SourceType, signal.Direction,
		signal.Strength, signal.Price, signal.TargetPrice, signal.StopLossPrice,
		signal.Period, signal.Status, signal.ConfirmedAt, signal.ExpiredAt,
		signal.TriggeredAt, signal.NotificationSent, signal.KlineTime,
	).Scan(&signal.ID)
	if err != nil {
		return fmt.Errorf("创建信号失败: %w", err)
	}

	return nil
}

func (r *SignalRepoPG) UpdateStatus(id int, status string) error {
	query := `UPDATE signals SET status = $1 WHERE id = $2`
	_, err := r.db.Exec(context.Background(), query, status, id)
	if err != nil {
		return fmt.Errorf("更新信号状态失败: %w", err)
	}
	return nil
}

func (r *SignalRepoPG) SetTriggeredAt(id int, triggeredAt *time.Time) error {
	query := `UPDATE signals SET triggered_at = $1 WHERE id = $2`
	_, err := r.db.Exec(context.Background(), query, triggeredAt, id)
	if err != nil {
		return fmt.Errorf("更新信号触发时间失败: %w", err)
	}
	return nil
}

func (r *SignalRepoPG) Update(signal *models.Signal) error {
	query := `
		UPDATE signals SET
			signal_type = $1, source_type = $2, direction = $3, strength = $4, price = $5,
			target_price = $6, stop_loss_price = $7, period = $8, status = $9,
			confirmed_at = $10, expired_at = $11, triggered_at = $12, notification_sent = $13
		WHERE id = $14
	`

	_, err := r.db.Exec(context.Background(), query,
		signal.SignalType, signal.SourceType, signal.Direction, signal.Strength,
		signal.Price, signal.TargetPrice, signal.StopLossPrice, signal.Period,
		signal.Status, signal.ConfirmedAt, signal.ExpiredAt, signal.TriggeredAt,
		signal.NotificationSent, signal.ID,
	)
	if err != nil {
		return fmt.Errorf("更新信号失败: %w", err)
	}

	return nil
}

func (r *SignalRepoPG) BatchUpdateByBatchID(batchID string, fields map[string]interface{}) error {
	// 暂时不实现
	return nil
}

func (r *SignalRepoPG) GetHistory(startDate, endDate time.Time, page, size int) ([]*models.Signal, int, error) {
	var signals []*models.Signal
	var count int

	// 计算总数
	countQuery := `
		SELECT COUNT(*)
		FROM signals s
		WHERE s.created_at BETWEEN $1 AND $2
	`

	err := r.db.QueryRow(context.Background(), countQuery, startDate, endDate).Scan(&count)
	if err != nil {
		return nil, 0, fmt.Errorf("查询信号总数失败: %w", err)
	}

	// 查询分页数据，JOIN symbols 表获取 symbol_code
	offset := (page - 1) * size
	dataQuery := `
		SELECT s.id, s.symbol_id, s.signal_type, s.source_type, s.direction, s.strength, s.price,
		       s.target_price, s.stop_loss_price, s.period, s.status, s.confirmed_at, s.expired_at,
		       s.triggered_at, s.notification_sent, s.created_at,
		       COALESCE(sy.symbol_code, '') as symbol_code
		FROM signals s
		LEFT JOIN symbols sy ON s.symbol_id = sy.id
		WHERE s.created_at BETWEEN $1 AND $2
		ORDER BY s.created_at DESC
		LIMIT $3 OFFSET $4
	`

	rows, err := r.db.Query(context.Background(), dataQuery, startDate, endDate, size, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("查询信号历史失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var signal models.Signal
		if err := rows.Scan(
			&signal.ID, &signal.SymbolID, &signal.SignalType, &signal.SourceType,
			&signal.Direction, &signal.Strength, &signal.Price, &signal.TargetPrice,
			&signal.StopLossPrice, &signal.Period, &signal.Status, &signal.ConfirmedAt,
			&signal.ExpiredAt, &signal.TriggeredAt, &signal.NotificationSent, &signal.CreatedAt,
			&signal.SymbolCode,
		); err != nil {
			return nil, 0, fmt.Errorf("扫描信号历史数据失败: %w", err)
		}
		signals = append(signals, &signal)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("遍历信号历史结果失败: %w", err)
	}

	return signals, count, nil
}

// Query 通用信号查询，支持多条件筛选
func (r *SignalRepoPG) Query(query *models.SignalQuery) ([]*models.Signal, int, error) {
	var signals []*models.Signal
	var count int

	// 构建WHERE条件
	var conditions []string
	var args []interface{}
	argIndex := 1

	// 日期范围条件
	if query.StartDate != nil {
		conditions = append(conditions, fmt.Sprintf("s.created_at >= $%d", argIndex))
		args = append(args, *query.StartDate)
		argIndex++
	}
	if query.EndDate != nil {
		conditions = append(conditions, fmt.Sprintf("s.created_at <= $%d", argIndex))
		args = append(args, *query.EndDate)
		argIndex++
	}

	// 市场条件 - 需要JOIN symbols和markets表
	if query.Market != "" {
		conditions = append(conditions, fmt.Sprintf("m.market_code = $%d", argIndex))
		args = append(args, query.Market)
		argIndex++
	}

	// 策略来源条件
	if query.SourceType != "" {
		conditions = append(conditions, fmt.Sprintf("s.source_type = $%d", argIndex))
		args = append(args, query.SourceType)
		argIndex++
	}

	// 信号类型条件
	if query.SignalType != "" {
		conditions = append(conditions, fmt.Sprintf("s.signal_type = $%d", argIndex))
		args = append(args, query.SignalType)
		argIndex++
	}

	// 方向条件
	if query.Direction != "" {
		conditions = append(conditions, fmt.Sprintf("s.direction = $%d", argIndex))
		args = append(args, query.Direction)
		argIndex++
	}

	// 强度条件
	if query.Strength != nil {
		conditions = append(conditions, fmt.Sprintf("s.strength = $%d", argIndex))
		args = append(args, *query.Strength)
		argIndex++
	}

	// 状态条件
	if query.Status != "" {
		conditions = append(conditions, fmt.Sprintf("s.status = $%d", argIndex))
		args = append(args, query.Status)
		argIndex++
	}

	// 构建WHERE子句
	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + conditions[0]
		for i := 1; i < len(conditions); i++ {
			whereClause += " AND " + conditions[i]
		}
	}

	// 确定是否需要JOIN markets表
	needJoinMarket := query.Market != ""

	// 计算总数
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM signals s
		LEFT JOIN symbols sy ON s.symbol_id = sy.id
		%s
		%s
	`, func() string {
		if needJoinMarket {
			return "JOIN markets m ON sy.market_id = m.id"
		}
		return ""
	}(), whereClause)

	err := r.db.QueryRow(context.Background(), countQuery, args...).Scan(&count)
	if err != nil {
		return nil, 0, fmt.Errorf("查询信号总数失败: %w", err)
	}

	// 计算分页
	page := query.Page
	if page < 1 {
		page = 1
	}
	size := query.PageSize
	if size < 1 {
		size = 20
	}
	offset := (page - 1) * size

	// 查询分页数据
	dataQuery := fmt.Sprintf(`
		SELECT s.id, s.symbol_id, s.signal_type, s.source_type, s.direction, s.strength, s.price,
		       s.target_price, s.stop_loss_price, s.period, s.status, s.confirmed_at, s.expired_at,
		       s.triggered_at, s.notification_sent, s.kline_time, s.created_at,
		       COALESCE(sy.symbol_code, '') as symbol_code
		FROM signals s
		LEFT JOIN symbols sy ON s.symbol_id = sy.id
		%s
		%s
		ORDER BY s.created_at DESC
		LIMIT $%d OFFSET $%d
	`, func() string {
		if needJoinMarket {
			return "JOIN markets m ON sy.market_id = m.id"
		}
		return ""
	}(), whereClause, argIndex, argIndex+1)

	args = append(args, size, offset)

	rows, err := r.db.Query(context.Background(), dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("查询信号列表失败: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var signal models.Signal
		if err := rows.Scan(
			&signal.ID, &signal.SymbolID, &signal.SignalType, &signal.SourceType,
			&signal.Direction, &signal.Strength, &signal.Price, &signal.TargetPrice,
			&signal.StopLossPrice, &signal.Period, &signal.Status, &signal.ConfirmedAt,
			&signal.ExpiredAt, &signal.TriggeredAt, &signal.NotificationSent, &signal.KlineTime,
			&signal.CreatedAt, &signal.SymbolCode,
		); err != nil {
			return nil, 0, fmt.Errorf("扫描信号数据失败: %w", err)
		}
		signals = append(signals, &signal)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("遍历信号结果失败: %w", err)
	}

	return signals, count, nil
}

// CountByMarket 按市场统计信号数量
func (r *SignalRepoPG) CountByMarket(market string) (int, error) {
	var count int
	var query string
	var args []interface{}

	if market == "" {
		// 统计所有信号
		query = `SELECT COUNT(*) FROM signals WHERE status = 'pending'`
	} else {
		// 按市场统计，需要JOIN
		query = `
			SELECT COUNT(*)
			FROM signals s
			JOIN symbols sy ON s.symbol_id = sy.id
			JOIN markets m ON sy.market_id = m.id
			WHERE m.market_code = $1 AND s.status = 'pending'
		`
		args = append(args, market)
	}

	err := r.db.QueryRow(context.Background(), query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("按市场统计信号失败: %w", err)
	}
	return count, nil
}

// CountBySignalType 按信号类型统计信号数量
func (r *SignalRepoPG) CountBySignalType(signalType string) (int, error) {
	var count int
	var query string
	var args []interface{}

	if signalType == "" {
		// 统计所有pending信号
		query = `SELECT COUNT(*) FROM signals WHERE status = 'pending'`
	} else {
		query = `SELECT COUNT(*) FROM signals WHERE signal_type = $1 AND status = 'pending'`
		args = append(args, signalType)
	}

	err := r.db.QueryRow(context.Background(), query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("按信号类型统计信号失败: %w", err)
	}
	return count, nil
}

// CountBySourceType 按策略来源统计信号数量
func (r *SignalRepoPG) CountBySourceType(sourceType string) (int, error) {
	var count int
	var query string
	var args []interface{}

	if sourceType == "" {
		query = `SELECT COUNT(*) FROM signals WHERE status = 'pending'`
	} else {
		query = `SELECT COUNT(*) FROM signals WHERE source_type = $1 AND status = 'pending'`
		args = append(args, sourceType)
	}

	err := r.db.QueryRow(context.Background(), query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("按策略来源统计信号失败: %w", err)
	}
	return count, nil
}
