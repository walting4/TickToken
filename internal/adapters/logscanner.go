package adapters

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// LogEntry 日志中提取的 token 事件
type LogEntry struct {
	Timestamp        time.Time
	Tool             string
	Model            string
	PromptTokens     int
	CompletionTokens int
	Source           string
	RawLine          string
}

// LogScanner 日志扫描器
type LogScanner struct {
	homeDir string
}

// NewLogScanner 创建日志扫描器
func NewLogScanner() (*LogScanner, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("无法获取用户目录: %w", err)
	}
	return &LogScanner{homeDir: homeDir}, nil
}

// ScanAll 扫描所有已知日志目录
func (s *LogScanner) ScanAll() ([]LogEntry, error) {
	var allEntries []LogEntry

	// Claude Code 日志
	if entries, err := s.ScanClaudeCode(); err == nil {
		allEntries = append(allEntries, entries...)
	}

	// Codex CLI 日志
	if entries, err := s.ScanCodexCLI(); err == nil {
		allEntries = append(allEntries, entries...)
	}

	// Aider 日志
	if entries, err := s.ScanAider(); err == nil {
		allEntries = append(allEntries, entries...)
	}

	return allEntries, nil
}

// ScanClaudeCode 扫描 ~/.claude/ 目录
func (s *LogScanner) ScanClaudeCode() ([]LogEntry, error) {
	dir := filepath.Join(s.homeDir, ".claude")
	return s.scanJSONLDir(dir, "claude-code")
}

// ScanCodexCLI 扫描 ~/.codex/log/ 目录
func (s *LogScanner) ScanCodexCLI() ([]LogEntry, error) {
	dir := filepath.Join(s.homeDir, ".codex", "log")
	return s.scanJSONLDir(dir, "codex-cli")
}

// scanJSONLDir 扫描 JSONL 格式的日志目录
func (s *LogScanner) scanJSONLDir(dir string, toolName string) ([]LogEntry, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if err != nil || len(files) == 0 {
		return nil, nil
	}

	var entries []LogEntry

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			var raw map[string]interface{}
			if err := json.Unmarshal([]byte(line), &raw); err != nil {
				continue
			}

			entry := s.parseLogEntry(raw, toolName, line)
			if entry != nil {
				entries = append(entries, *entry)
			}
		}
	}

	log.Printf("[Adapters] 从 %s 扫描到 %d 条日志记录", dir, len(entries))
	return entries, nil
}

// parseLogEntry 从 JSON 日志中解析 token 信息
func (s *LogScanner) parseLogEntry(raw map[string]interface{}, toolName string, rawLine string) *LogEntry {
	entry := &LogEntry{
		Tool:    toolName,
		Source:  "log",
		RawLine: rawLine,
	}

	// 尝试提取时间戳
	if ts, ok := raw["timestamp"].(string); ok {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			entry.Timestamp = t
		}
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	// 尝试提取 model
	if model, ok := raw["model"].(string); ok {
		entry.Model = model
	}

	// 尝试提取 token usage
	if usage, ok := raw["usage"].(map[string]interface{}); ok {
		if pt, ok := usage["prompt_tokens"].(float64); ok {
			entry.PromptTokens = int(pt)
		}
		if ct, ok := usage["completion_tokens"].(float64); ok {
			entry.CompletionTokens = int(ct)
		}
		if it, ok := usage["input_tokens"].(float64); ok && entry.PromptTokens == 0 {
			entry.PromptTokens = int(it)
		}
		if ot, ok := usage["output_tokens"].(float64); ok && entry.CompletionTokens == 0 {
			entry.CompletionTokens = int(ot)
		}
	}

	// 只有在提取到 token 数据时才返回
	if entry.PromptTokens > 0 || entry.CompletionTokens > 0 {
		return entry
	}

	return nil
}

// ScanAider 扫描 .aider.chat.history.md 文件
func (s *LogScanner) ScanAider() ([]LogEntry, error) {
	// 查找当前目录及常见位置的 aider 日志
	candidates := []string{
		".aider.chat.history.md",
		filepath.Join(s.homeDir, ".aider.chat.history.md"),
	}

	var entries []LogEntry

	// aider 日志中 token 信息可能出现在行内
	tokenPattern := regexp.MustCompile(`(?:tokens|tokens used)[:\s]*(\d+)`)
	modelPattern := regexp.MustCompile(`model[:\s]+([a-zA-Z0-9\-_.]+)`)

	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		content := string(data)
		lines := strings.Split(content, "\n")

		var currentModel string
		for _, line := range lines {
			if m := modelPattern.FindStringSubmatch(line); m != nil {
				currentModel = m[1]
			}

			if tokens := tokenPattern.FindStringSubmatch(line); tokens != nil {
				entry := LogEntry{
					Timestamp: time.Now(),
					Tool:      "aider",
					Model:     currentModel,
					Source:    "log",
					RawLine:   line,
				}
				// 尝试解析数字
				var num int
				fmt.Sscanf(tokens[1], "%d", &num)
				entry.PromptTokens = num
				entries = append(entries, entry)
			}
		}

		if len(entries) > 0 {
			log.Printf("[Adapters] 从 %s 扫描到 %d 条 aider 日志记录", path, len(entries))
		}
	}

	return entries, nil
}
