package adapters

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// WatchEvent 实时监控捕获的事件
type WatchEvent struct {
	Source            string    // "claude-code" / "codex-cli" / "aider"
	Model             string
	PromptTokens      int
	CompletionTokens  int
	Timestamp         time.Time
	FilePath          string // 来源文件路径
	LineOffset        int64  // 文件偏移量（增量扫描进度）
}

// FileWatcher 文件监控器，实现 CLI 工具日志的实时增量扫描
// 相比 LogScanner 的一次性全量扫描，FileWatcher 持续监控日志文件的新增内容
type FileWatcher struct {
	mu        sync.Mutex
	homeDir   string
	watchers  map[string]*fileTrack // 文件路径 -> 追踪状态
	stopCh    chan struct{}
	eventCh   chan WatchEvent
	interval  time.Duration
	running   bool
}

// fileTrack 单个文件的追踪状态
type fileTrack struct {
	path     string
	toolName string
	offset   int64 // 已读取的文件偏移量
}

// NewFileWatcher 创建文件监控器
// interval 为扫描间隔（建议 500ms - 2s）
func NewFileWatcher(interval time.Duration) (*FileWatcher, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("无法获取用户目录: %w", err)
	}

	if interval <= 0 {
		interval = 1 * time.Second
	}

	return &FileWatcher{
		homeDir:  homeDir,
		watchers: make(map[string]*fileTrack),
		stopCh:   make(chan struct{}),
		eventCh:  make(chan WatchEvent, 256),
		interval: interval,
	}, nil
}

// Events 返回事件通道（调用方应消费此通道）
func (w *FileWatcher) Events() <-chan WatchEvent {
	return w.eventCh
}

// Start 启动监控
func (w *FileWatcher) Start() error {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return fmt.Errorf("监控器已在运行")
	}
	w.running = true
	w.mu.Unlock()

	// 注册监控目标
	w.registerTargets()

	go w.pollLoop()
	log.Printf("[Watcher] CLI 工具日志监控已启动，间隔 %v", w.interval)
	return nil
}

// Stop 停止监控
func (w *FileWatcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.running {
		return
	}
	close(w.stopCh)
	w.running = false
}

// registerTargets 注册所有监控目标
func (w *FileWatcher) registerTargets() {
	targets := []struct {
		dir      string
		toolName string
	}{
		{filepath.Join(w.homeDir, ".claude"), "claude-code"},
		{filepath.Join(w.homeDir, ".codex", "log"), "codex-cli"},
		{filepath.Join(w.homeDir, ".aider"), "aider"},
	}

	for _, t := range targets {
		w.scanDir(t.dir, t.toolName)
	}
}

// scanDir 扫描目录，注册匹配的文件
func (w *FileWatcher) scanDir(dir, toolName string) {
	// JSONL 文件
	patterns := []string{"*.jsonl", "*.log", "*.json"}
	for _, pattern := range patterns {
		files, err := filepath.Glob(filepath.Join(dir, pattern))
		if err != nil {
			continue
		}
		for _, f := range files {
			w.registerFile(f, toolName)
		}
	}

	// aider 历史文件
	if toolName == "aider" {
		historyFile := filepath.Join(w.homeDir, ".aider.chat.history.md")
		if _, err := os.Stat(historyFile); err == nil {
			w.registerFile(historyFile, "aider")
		}
	}
}

// registerFile 注册单个文件进行监控
func (w *FileWatcher) registerFile(path, toolName string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, exists := w.watchers[path]; exists {
		return
	}

	// 从文件末尾开始监控（只读新增内容）
	info, err := os.Stat(path)
	if err != nil {
		return
	}

	w.watchers[path] = &fileTrack{
		path:     path,
		toolName: toolName,
		offset:   info.Size(), // 从当前末尾开始
	}
}

// pollLoop 轮询循环
func (w *FileWatcher) pollLoop() {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	// 定期重新扫描目录以发现新文件
	rescanTicker := time.NewTicker(10 * time.Second)
	defer rescanTicker.Stop()

	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.pollOnce()
		case <-rescanTicker.C:
			w.registerTargets()
		}
	}
}

// pollOnce 执行一次轮询
func (w *FileWatcher) pollOnce() {
	w.mu.Lock()
	// 复制 watchers 列表避免长时间持锁
	tracks := make([]*fileTrack, 0, len(w.watchers))
	for _, t := range w.watchers {
		tracks = append(tracks, t)
	}
	w.mu.Unlock()

	for _, t := range tracks {
		w.checkFile(t)
	}
}

// checkFile 检查单个文件是否有新内容
func (w *FileWatcher) checkFile(t *fileTrack) {
	info, err := os.Stat(t.path)
	if err != nil {
		// 文件可能被删除或轮转
		return
	}

	// 文件被截断/轮转，重置偏移
	if info.Size() < t.offset {
		t.offset = 0
	}

	// 无新内容
	if info.Size() == t.offset {
		return
	}

	// 读取新增内容
	f, err := os.Open(t.path)
	if err != nil {
		return
	}
	defer f.Close()

	if _, err := f.Seek(t.offset, 0); err != nil {
		return
	}

	buf := make([]byte, info.Size()-t.offset)
	if _, err := f.Read(buf); err != nil {
		return
	}

	t.offset = info.Size()

	// 解析新内容
	w.parseNewContent(buf, t)
}

// parseNewContent 解析新增内容
func (w *FileWatcher) parseNewContent(data []byte, t *fileTrack) {
	lines := splitLines(data)
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}

		// 尝试 JSON 解析
		event := w.parseJSONLine(line, t)
		if event != nil {
			w.emitEvent(event)
			continue
		}

		// 尝试 aider markdown 格式
		event = w.parseAiderLine(line, t)
		if event != nil {
			w.emitEvent(event)
		}
	}
}

// parseJSONLine 解析 JSON 格式的日志行
func (w *FileWatcher) parseJSONLine(line []byte, t *fileTrack) *WatchEvent {
	var raw map[string]interface{}
	if err := json.Unmarshal(line, &raw); err != nil {
		return nil
	}

	event := &WatchEvent{
		Source:    t.toolName,
		Timestamp: time.Now(),
		FilePath:  t.path,
	}

	// 时间戳
	if ts, ok := raw["timestamp"].(string); ok {
		if parsed, err := time.Parse(time.RFC3339, ts); err == nil {
			event.Timestamp = parsed
		}
	}

	// 模型
	if model, ok := raw["model"].(string); ok {
		event.Model = model
	}

	// token usage（OpenAI 风格）
	if usage, ok := raw["usage"].(map[string]interface{}); ok {
		if pt, ok := usage["prompt_tokens"].(float64); ok {
			event.PromptTokens = int(pt)
		}
		if ct, ok := usage["completion_tokens"].(float64); ok {
			event.CompletionTokens = int(ct)
		}
		// Anthropic 风格
		if it, ok := usage["input_tokens"].(float64); ok && event.PromptTokens == 0 {
			event.PromptTokens = int(it)
		}
		if ot, ok := usage["output_tokens"].(float64); ok && event.CompletionTokens == 0 {
			event.CompletionTokens = int(ot)
		}
	}

	// 只有提取到 token 数据才返回
	if event.PromptTokens > 0 || event.CompletionTokens > 0 {
		return event
	}

	return nil
}

// parseAiderLine 解析 aider markdown 格式
func (w *FileWatcher) parseAiderLine(line []byte, t *fileTrack) *WatchEvent {
	// aider 日志中 token 信息格式: "tokens: 1234" 或 "tokens used: 1234"
	// 这里使用简单字符串匹配
	// 完整实现见 logscanner.go 中的正则
	return nil // 简化：aider 实时解析委托给定期全量扫描
}

// emitEvent 发送事件到通道
func (w *FileWatcher) emitEvent(event *WatchEvent) {
	select {
	case w.eventCh <- *event:
	default:
		// 通道满，丢弃事件（避免阻塞监控循环）
		log.Printf("[Watcher] 事件通道已满，丢弃事件")
	}
}

// splitLines 按行分割（避免引入 bufio）
func splitLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i, b := range data {
		if b == '\n' {
			lines = append(lines, data[start:i])
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}

// GetWatchedFiles 返回当前监控的文件列表
func (w *FileWatcher) GetWatchedFiles() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	files := make([]string, 0, len(w.watchers))
	for path := range w.watchers {
		files = append(files, path)
	}
	return files
}
