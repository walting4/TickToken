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
		tokenizer TEXT NOT NULL DEFAULT ''
	);
	CREATE INDEX IF NOT EXISTS idx_events_timestamp ON token_events(timestamp);
	CREATE INDEX IF NOT EXISTS idx_events_tool ON token_events(tool);
	CREATE INDEX IF NOT EXISTS idx_events_model ON token_events(model);
	`
	_, err := s.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("初始化数据库失败: %w", err)
	}
	log.Println("[Storage] SQLite 存储已初始化")
	return nil
}

// InsertEvent 写入一条事件
func (s *Store) InsertEvent(event *TokenEvent) error {
	_, err := s.db.Exec(
		`INSERT INTO token_events (timestamp, tool, model, prompt_tokens, completion_tokens, cache_hit, cache_miss, cache_creation, source, tokenizer)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.Timestamp, event.Tool, event.Model,
		event.PromptTokens, event.CompletionTokens,
		event.CacheHit, event.CacheMiss, event.CacheCreation,
		event.Source, event.Tokenizer,
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
	query := `SELECT id, timestamp, tool, model, prompt_tokens, completion_tokens, cache_hit, cache_miss, cache_creation, source, tokenizer FROM token_events WHERE 1=1`
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
		if err := rows.Scan(&e.ID, &e.Timestamp, &e.Tool, &e.Model, &e.PromptTokens, &e.CompletionTokens, &e.CacheHit, &e.CacheMiss, &e.CacheCreation, &e.Source, &e.Tokenizer); err != nil {
			continue
		}
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
