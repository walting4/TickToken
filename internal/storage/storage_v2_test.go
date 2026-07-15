package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("创建测试存储失败: %v", err)
	}
	return store
}

// TestInsertAndQueryEvent 测试事件插入和查询
func TestInsertAndQueryEvent(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	event := &TokenEvent{
		Timestamp:        time.Now(),
		Tool:             "vscode",
		Model:            "gpt-4o",
		PromptTokens:     100,
		CompletionTokens: 50,
		CacheHit:         80,
		CacheMiss:        20,
		Source:           "response",
		Tokenizer:        "none",
		Provider:         "openai",
	}

	if err := store.InsertEvent(event); err != nil {
		t.Fatalf("插入事件失败: %v", err)
	}

	events, err := store.Query(QueryFilter{Tool: "vscode"})
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("期望 1 条事件，得到 %d", len(events))
	}
	if events[0].PromptTokens != 100 {
		t.Errorf("PromptTokens 期望 100，得到 %d", events[0].PromptTokens)
	}
	if events[0].Provider != "openai" {
		t.Errorf("Provider 期望 openai，得到 %s", events[0].Provider)
	}
}

// TestInsertEventWithAnomaly 测试异常事件存储
func TestInsertEventWithAnomaly(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	event := &TokenEvent{
		Timestamp:    time.Now(),
		Tool:         "cursor",
		Model:        "claude-3-5-sonnet",
		PromptTokens: 100,
		IsAnomaly:    true,
		AnomalyType:  "prompt_deviation",
		DeviationPct: 35.5,
		LatencyMs:    2000,
		Provider:     "anthropic",
	}

	if err := store.InsertEvent(event); err != nil {
		t.Fatalf("插入异常事件失败: %v", err)
	}

	anomalies, err := store.QueryAnomalies(10)
	if err != nil {
		t.Fatalf("查询异常失败: %v", err)
	}
	if len(anomalies) != 1 {
		t.Fatalf("期望 1 条异常，得到 %d", len(anomalies))
	}
	if !anomalies[0].IsAnomaly {
		t.Error("事件应标记为异常")
	}
	if anomalies[0].AnomalyType != "prompt_deviation" {
		t.Errorf("AnomalyType 期望 prompt_deviation，得到 %s", anomalies[0].AnomalyType)
	}
}

// TestQueryTrend 测试趋势分析查询
func TestQueryTrend(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	now := time.Now()
	// 插入 3 条事件，分布在不同小时
	for i := 0; i < 3; i++ {
		event := &TokenEvent{
			Timestamp:        now.Add(-time.Duration(i) * time.Hour),
			Tool:             "vscode",
			Model:            "gpt-4o",
			PromptTokens:     100 * (i + 1),
			CompletionTokens: 50 * (i + 1),
			Source:           "response",
		}
		if err := store.InsertEvent(event); err != nil {
			t.Fatalf("插入事件失败: %v", err)
		}
	}

	start := now.Add(-4 * time.Hour)
	end := now
	points, err := store.QueryTrend(start, end, "hour")
	if err != nil {
		t.Fatalf("查询趋势失败: %v", err)
	}
	if len(points) == 0 {
		t.Fatal("应返回趋势数据点")
	}

	// 验证数据点包含 token 总量
	var totalTokens int
	for _, p := range points {
		totalTokens += p.TotalTokens
	}
	if totalTokens == 0 {
		t.Error("趋势数据 token 总量不应为 0")
	}
}

// TestQueryAnomalyStats 测试异常统计
func TestQueryAnomalyStats(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// 插入正常事件
	for i := 0; i < 8; i++ {
		store.InsertEvent(&TokenEvent{
			Timestamp:    time.Now(),
			Tool:         "vscode",
			Model:        "gpt-4o",
			PromptTokens: 100,
			IsAnomaly:    false,
		})
	}
	// 插入异常事件
	for i := 0; i < 2; i++ {
		store.InsertEvent(&TokenEvent{
			Timestamp:    time.Now(),
			Tool:         "cursor",
			Model:        "claude-3",
			PromptTokens: 100,
			IsAnomaly:    true,
			AnomalyType:  "prompt_deviation",
			DeviationPct: 30.0,
		})
	}

	start := time.Now().Add(-1 * time.Hour)
	end := time.Now()
	stats, err := store.QueryAnomalyStats(start, end)
	if err != nil {
		t.Fatalf("查询异常统计失败: %v", err)
	}
	if stats.TotalAnomalies != 2 {
		t.Errorf("TotalAnomalies 期望 2，得到 %d", stats.TotalAnomalies)
	}
	if stats.AnomalyRate != 20.0 {
		t.Errorf("AnomalyRate 期望 20%%，得到 %.2f%%", stats.AnomalyRate)
	}
	if stats.ByType["prompt_deviation"] != 2 {
		t.Errorf("ByType[prompt_deviation] 期望 2，得到 %d", stats.ByType["prompt_deviation"])
	}
}

// TestQueryByDimension 测试按维度聚合
func TestQueryByDimension(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// 插入不同模型的事件
	store.InsertEvent(&TokenEvent{
		Timestamp: time.Now(), Tool: "vscode", Model: "gpt-4o",
		PromptTokens: 100, CompletionTokens: 50, Source: "response",
	})
	store.InsertEvent(&TokenEvent{
		Timestamp: time.Now(), Tool: "vscode", Model: "gpt-4o",
		PromptTokens: 200, CompletionTokens: 100, Source: "response",
	})
	store.InsertEvent(&TokenEvent{
		Timestamp: time.Now(), Tool: "cursor", Model: "claude-3",
		PromptTokens: 150, CompletionTokens: 75, Source: "response",
	})

	start := time.Now().Add(-1 * time.Hour)
	end := time.Now()

	// 按模型聚合
	items, err := store.QueryByDimension("model", start, end)
	if err != nil {
		t.Fatalf("按模型聚合失败: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("期望 2 个模型，得到 %d", len(items))
	}

	// 按工具聚合
	items, err = store.QueryByDimension("tool", start, end)
	if err != nil {
		t.Fatalf("按工具聚合失败: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("期望 2 个工具，得到 %d", len(items))
	}
}

// TestQueryTimeSeries 测试时间序列查询
func TestQueryTimeSeries(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	now := time.Now()
	store.InsertEvent(&TokenEvent{
		Timestamp: now, Tool: "vscode", Model: "gpt-4o",
		PromptTokens: 100, CompletionTokens: 50, Source: "response",
	})

	start := now.Add(-1 * time.Hour)
	end := now.Add(1 * time.Minute)

	points, err := store.QueryTimeSeries(start, end)
	if err != nil {
		t.Fatalf("查询时间序列失败: %v", err)
	}
	if len(points) == 0 {
		t.Fatal("应返回时间序列数据")
	}
	if points[0].PromptTokens != 100 {
		t.Errorf("PromptTokens 期望 100，得到 %d", points[0].PromptTokens)
	}
}

// TestStoreMigration 测试数据库迁移（旧表添加新列）
func TestStoreMigration(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	// 第一次创建
	store1, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("第一次创建失败: %v", err)
	}
	store1.Close()

	// 第二次打开（应执行迁移但不报错）
	store2, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("第二次打开失败: %v", err)
	}
	defer store2.Close()

	// 验证新列可用
	err = store2.InsertEvent(&TokenEvent{
		Timestamp:   time.Now(),
		Tool:        "test",
		Model:       "test",
		IsAnomaly:   true,
		AnomalyType: "test_type",
		Provider:    "test_provider",
	})
	if err != nil {
		t.Fatalf("迁移后插入失败: %v", err)
	}
}

// TestStoreConcurrentInsert 测试并发写入
func TestStoreConcurrentInsert(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	done := make(chan bool, 50)
	for i := 0; i < 50; i++ {
		go func(n int) {
			err := store.InsertEvent(&TokenEvent{
				Timestamp:    time.Now(),
				Tool:         "vscode",
				Model:        "gpt-4o",
				PromptTokens: n,
				IsAnomaly:    n%10 == 0,
			})
			if err != nil {
				t.Errorf("并发插入失败: %v", err)
			}
			done <- true
		}(i)
	}

	for i := 0; i < 50; i++ {
		<-done
	}

	events, err := store.Query(QueryFilter{})
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if len(events) != 50 {
		t.Errorf("期望 50 条事件，得到 %d", len(events))
	}
}

// TestDBFileCreation 测试数据库文件实际创建
func TestDBFileCreation(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("创建存储失败: %v", err)
	}
	store.Close()

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("数据库文件应被创建")
	}
}
