package storage

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// TokenEvent token 事件记录
type TokenEvent struct {
	ID               int64
	Timestamp        time.Time
	Tool             string // 来源工具（如 "vscode"、"cursor"、"trae"、"unknown"）
	Model            string // 模型名称
	PromptTokens     int
	CompletionTokens int
	CacheHit         int
	CacheMiss        int
	CacheCreation    int
	Source           string // 计数来源 "response" 或 "local_tokenizer"
	Tokenizer        string // 使用的 tokenizer
	// 优化新增字段（v2）
	IsAnomaly        bool    // 是否被标记为异常
	AnomalyType      string  // 异常类型（"prompt_deviation" / "missing_usage" 等）
	DeviationPct     float64 // 本地估算与响应 usage 的偏差百分比
	LatencyMs        int64   // 请求延迟（毫秒）
	Provider         string  // API provider（"openai" / "anthropic" / "gemini" 等）
}

// Store SQLite 存储
type Store struct {
	db *sql.DB
}

// NewStore 创建存储实例
func NewStore(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	// 优化设置
	db.SetMaxOpenConns(1) // SQLite 单写
	db.SetMaxIdleConns(1)

	s := &Store{db: db}
	if err := s.init(); err != nil {
		return nil, err
	}

	return s, nil
}

// init 初始化表结构
func (s *Store) init() error {
	schema := `
	CREATE TABLE IF NOT EXISTS token_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME NOT NULL,
		tool TEXT NOT NULL DEFAULT 'unknown',
		model TEXT NOT NULL DEFAULT '',
		prompt_tokens INTEGER NOT NULL DEFAULT 0,
		completion_tokens INTEGER NOT NULL DEFAULT 0,
		cache_hit INTEGER NOT NULL DEFAULT 0,
		cache_miss INTEGER NOT NULL DEFAULT 0,
		cache_creation INTEGER NOT NULL DEFAULT 0,
		source TEXT NOT NULL DEFAULT '',
		tokenizer TEXT NOT NULL DEFAULT '',
		is_anomaly INTEGER NOT NULL DEFAULT 0,
		anomaly_type TEXT NOT NULL DEFAULT '',
		deviation_pct REAL NOT NULL DEFAULT 0,
		latency_ms INTEGER NOT NULL DEFAULT 0,
		provider TEXT NOT NULL DEFAULT ''
	);
	CREATE INDEX IF NOT EXISTS idx_events_timestamp ON token_events(timestamp);
	CREATE INDEX IF NOT EXISTS idx_events_tool ON token_events(tool);
	CREATE INDEX IF NOT EXISTS idx_events_model ON token_events(model);
	CREATE INDEX IF NOT EXISTS idx_events_anomaly ON token_events(is_anomaly);
	CREATE INDEX IF NOT EXISTS idx_events_provider ON token_events(provider);
	`
	_, err := s.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("初始化数据库失败: %w", err)
	}

	// 迁移：为旧表添加新列（如果不存在）
	migrations := []string{
		`ALTER TABLE token_events ADD COLUMN is_anomaly INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE token_events ADD COLUMN anomaly_type TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE token_events ADD COLUMN deviation_pct REAL NOT NULL DEFAULT 0`,
		`ALTER TABLE token_events ADD COLUMN latency_ms INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE token_events ADD COLUMN provider TEXT NOT NULL DEFAULT ''`,
	}
	for _, m := range migrations {
		s.db.Exec(m) // 忽略错误（列已存在）
	}

	log.Println("[Storage] SQLite 存储已初始化")
	return nil
}

// InsertEvent 写入一条事件
func (s *Store) InsertEvent(event *TokenEvent) error {
	_, err := s.db.Exec(
		`INSERT INTO token_events (timestamp, tool, model, prompt_tokens, completion_tokens, cache_hit, cache_miss, cache_creation, source, tokenizer, is_anomaly, anomaly_type, deviation_pct, latency_ms, provider)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.Timestamp, event.Tool, event.Model,
		event.PromptTokens, event.CompletionTokens,
		event.CacheHit, event.CacheMiss, event.CacheCreation,
		event.Source, event.Tokenizer,
		event.IsAnomaly, event.AnomalyType, event.DeviationPct, event.LatencyMs, event.Provider,
	)
	if err != nil {
		return fmt.Errorf("写入事件失败: %w", err)
	}
	return nil
}

// QueryFilter 查询过滤条件
type QueryFilter struct {
	StartTime *time.Time
	EndTime   *time.Time
	Tool      string
	Model     string
}

// Query 按条件查询事件
func (s *Store) Query(filter QueryFilter) ([]TokenEvent, error) {
	query := `SELECT id, timestamp, tool, model, prompt_tokens, completion_tokens, cache_hit, cache_miss, cache_creation, source, tokenizer, is_anomaly, anomaly_type, deviation_pct, latency_ms, provider FROM token_events WHERE 1=1`
	args := []interface{}{}

	if filter.StartTime != nil {
		query += " AND timestamp >= ?"
		args = append(args, *filter.StartTime)
	}
	if filter.EndTime != nil {
		query += " AND timestamp <= ?"
		args = append(args, *filter.EndTime)
	}
	if filter.Tool != "" {
		query += " AND tool = ?"
		args = append(args, filter.Tool)
	}
	if filter.Model != "" {
		query += " AND model = ?"
		args = append(args, filter.Model)
	}

	query += " ORDER BY timestamp DESC LIMIT 10000"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询失败: %w", err)
	}
	defer rows.Close()

	var events []TokenEvent
	for rows.Next() {
		var e TokenEvent
		var isAnomaly int
		if err := rows.Scan(&e.ID, &e.Timestamp, &e.Tool, &e.Model, &e.PromptTokens, &e.CompletionTokens, &e.CacheHit, &e.CacheMiss, &e.CacheCreation, &e.Source, &e.Tokenizer, &isAnomaly, &e.AnomalyType, &e.DeviationPct, &e.LatencyMs, &e.Provider); err != nil {
			continue
		}
		e.IsAnomaly = isAnomaly == 1
		events = append(events, e)
	}

	return events, nil
}

// TimeSeriesPoint 时间序列数据点
type TimeSeriesPoint struct {
	Timestamp        time.Time
	PromptTokens     int
	CompletionTokens int
	Total            int
}

// QueryTimeSeries 查询时间序列数据（按小时聚合）
func (s *Store) QueryTimeSeries(startTime, endTime time.Time) ([]TimeSeriesPoint, error) {
	query := `
	SELECT 
		strftime('%Y-%m-%d %H:00:00', timestamp) as hour,
		SUM(prompt_tokens) as prompt_tokens,
		SUM(completion_tokens) as completion_tokens,
		SUM(prompt_tokens + completion_tokens) as total
	FROM token_events
	WHERE timestamp >= ? AND timestamp <= ?
	GROUP BY hour
	ORDER BY hour
	`

	rows, err := s.db.Query(query, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("查询时间序列失败: %w", err)
	}
	defer rows.Close()

	var points []TimeSeriesPoint
	for rows.Next() {
		var p TimeSeriesPoint
		var hourStr string
		if err := rows.Scan(&hourStr, &p.PromptTokens, &p.CompletionTokens, &p.Total); err != nil {
			continue
		}
		p.Timestamp, _ = time.Parse("2006-01-02 15:00:00", hourStr)
		points = append(points, p)
	}

	return points, nil
}

// AggregationItem 聚合数据项
type AggregationItem struct {
	Key              string // 模型名/工具名/缓存状态
	PromptTokens     int
	CompletionTokens int
	CacheHit         int
	CacheMiss        int
	CacheCreation    int
	Total            int
}

// QueryByDimension 按维度聚合查询（model/tool）
func (s *Store) QueryByDimension(dimension string, startTime, endTime time.Time) ([]AggregationItem, error) {
	if dimension != "model" && dimension != "tool" {
		return nil, fmt.Errorf("不支持的维度: %s", dimension)
	}

	query := fmt.Sprintf(`
	SELECT 
		%s as key,
		SUM(prompt_tokens) as prompt_tokens,
		SUM(completion_tokens) as completion_tokens,
		SUM(cache_hit) as cache_hit,
		SUM(cache_miss) as cache_miss,
		SUM(cache_creation) as cache_creation,
		SUM(prompt_tokens + completion_tokens) as total
	FROM token_events
	WHERE timestamp >= ? AND timestamp <= ?
	GROUP BY %s
	ORDER BY total DESC
	LIMIT 50
	`, dimension, dimension)

	rows, err := s.db.Query(query, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("查询聚合失败: %w", err)
	}
	defer rows.Close()

	var items []AggregationItem
	for rows.Next() {
		var item AggregationItem
		if err := rows.Scan(&item.Key, &item.PromptTokens, &item.CompletionTokens, &item.CacheHit, &item.CacheMiss, &item.CacheCreation, &item.Total); err != nil {
			continue
		}
		items = append(items, item)
	}

	return items, nil
}

// Close 关闭数据库
func (s *Store) Close() error {
	return s.db.Close()
}

// TrendPoint 趋势分析数据点
type TrendPoint struct {
	Timestamp        time.Time
	TotalTokens      int
	PromptTokens     int
	CompletionTokens int
	EventCount       int
	AnomalyCount     int
}

// QueryTrend 查询趋势分析数据（按指定粒度聚合）
// granularity: "hour" 或 "day"
func (s *Store) QueryTrend(startTime, endTime time.Time, granularity string) ([]TrendPoint, error) {
	var format string
	switch granularity {
	case "day":
		format = "%Y-%m-%d"
	case "hour":
		format = "%Y-%m-%d %H:00:00"
	default:
		format = "%Y-%m-%d %H:00:00"
	}

	query := fmt.Sprintf(`
	SELECT
		strftime('%s', timestamp) as bucket,
		SUM(prompt_tokens + completion_tokens) as total_tokens,
		SUM(prompt_tokens) as prompt_tokens,
		SUM(completion_tokens) as completion_tokens,
		COUNT(*) as event_count,
		SUM(CASE WHEN is_anomaly = 1 THEN 1 ELSE 0 END) as anomaly_count
	FROM token_events
	WHERE timestamp >= ? AND timestamp <= ?
	GROUP BY bucket
	ORDER BY bucket
	`, format)

	rows, err := s.db.Query(query, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("查询趋势失败: %w", err)
	}
	defer rows.Close()

	var points []TrendPoint
	parseFormat := "2006-01-02"
	if granularity == "hour" {
		parseFormat = "2006-01-02 15:00:00"
	}

	for rows.Next() {
		var p TrendPoint
		var bucketStr string
		if err := rows.Scan(&bucketStr, &p.TotalTokens, &p.PromptTokens, &p.CompletionTokens, &p.EventCount, &p.AnomalyCount); err != nil {
			continue
		}
		p.Timestamp, _ = time.Parse(parseFormat, bucketStr)
		points = append(points, p)
	}

	return points, nil
}

// AnomalyStats 异常统计
type AnomalyStats struct {
	TotalAnomalies   int64   // 异常总数
	AnomalyRate      float64 // 异常率（百分比）
	ByType           map[string]int64 // 按异常类型统计
	AvgDeviation     float64 // 平均偏差
}

// QueryAnomalyStats 查询指定时间窗口的异常统计
func (s *Store) QueryAnomalyStats(startTime, endTime time.Time) (*AnomalyStats, error) {
	query := `
	SELECT
		COUNT(*) as total,
		SUM(CASE WHEN is_anomaly = 1 THEN 1 ELSE 0 END) as anomalies,
		AVG(CASE WHEN is_anomaly = 1 THEN deviation_pct ELSE NULL END) as avg_dev
	FROM token_events
	WHERE timestamp >= ? AND timestamp <= ?
	`
	var total, anomalies int64
	var avgDev sql.NullFloat64

	err := s.db.QueryRow(query, startTime, endTime).Scan(&total, &anomalies, &avgDev)
	if err != nil {
		return nil, fmt.Errorf("查询异常统计失败: %w", err)
	}

	stats := &AnomalyStats{
		TotalAnomalies: anomalies,
		ByType:         make(map[string]int64),
	}
	if total > 0 {
		stats.AnomalyRate = float64(anomalies) / float64(total) * 100.0
	}
	if avgDev.Valid {
		stats.AvgDeviation = avgDev.Float64
	}

	// 按类型统计
	typeQuery := `
	SELECT anomaly_type, COUNT(*) as cnt
	FROM token_events
	WHERE timestamp >= ? AND timestamp <= ? AND is_anomaly = 1
	GROUP BY anomaly_type
	`
	typeRows, err := s.db.Query(typeQuery, startTime, endTime)
	if err == nil {
		defer typeRows.Close()
		for typeRows.Next() {
			var aType string
			var cnt int64
			if typeRows.Scan(&aType, &cnt) == nil {
				stats.ByType[aType] = cnt
			}
		}
	}

	return stats, nil
}

// QueryAnomalies 查询异常事件列表
func (s *Store) QueryAnomalies(limit int) ([]TokenEvent, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	query := `
	SELECT id, timestamp, tool, model, prompt_tokens, completion_tokens, cache_hit, cache_miss, cache_creation, source, tokenizer, is_anomaly, anomaly_type, deviation_pct, latency_ms, provider
	FROM token_events
	WHERE is_anomaly = 1
	ORDER BY timestamp DESC
	LIMIT ?
	`
	rows, err := s.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("查询异常事件失败: %w", err)
	}
	defer rows.Close()

	var events []TokenEvent
	for rows.Next() {
		var e TokenEvent
		var isAnomaly int
		if err := rows.Scan(&e.ID, &e.Timestamp, &e.Tool, &e.Model, &e.PromptTokens, &e.CompletionTokens, &e.CacheHit, &e.CacheMiss, &e.CacheCreation, &e.Source, &e.Tokenizer, &isAnomaly, &e.AnomalyType, &e.DeviationPct, &e.LatencyMs, &e.Provider); err != nil {
			continue
		}
		e.IsAnomaly = isAnomaly == 1
		events = append(events, e)
	}

	return events, nil
}
