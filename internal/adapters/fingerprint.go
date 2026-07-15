package adapters

import (
	"net/http"
	"strings"
)

// Fingerprint 工具指纹
type Fingerprint struct {
	ToolName string   // 工具名称
	Patterns []string // User-Agent 或目标域名匹配模式（小写）
}

// FingerprintDB 指纹数据库
type FingerprintDB struct {
	fingerprints []Fingerprint
}

// NewFingerprintDB 创建指纹数据库，内置常见工具指纹
func NewFingerprintDB() *FingerprintDB {
	return &FingerprintDB{
		fingerprints: []Fingerprint{
			{
				ToolName: "vscode",
				Patterns: []string{"vscode", "vs code", "github copilot", "visual studio code"},
			},
			{
				ToolName: "cursor",
				Patterns: []string{"cursor"},
			},
			{
				ToolName: "jetbrains",
				Patterns: []string{"jetbrains", "intellij", "pycharm", "webstorm", "goland", "rustrover", "clion", "phpstorm"},
			},
			{
				ToolName: "windsurf",
				Patterns: []string{"windsurf", "codeium"},
			},
			{
				ToolName: "trae",
				Patterns: []string{"trae"},
			},
			{
				ToolName: "workbuddy",
				Patterns: []string{"workbuddy", "work-buddy"},
			},
			{
				ToolName: "cline",
				Patterns: []string{"cline"},
			},
			{
				ToolName: "aider",
				Patterns: []string{"aider"},
			},
			{
				ToolName: "claude-code",
				Patterns: []string{"claude-code", "claude code"},
			},
			{
				ToolName: "codex-cli",
				Patterns: []string{"codex", "openai-cli"},
			},
			{
				ToolName: "continue",
				Patterns: []string{"continue-dev", "continue"},
			},
			{
				ToolName: "copilot-cli",
				Patterns: []string{"github-copilot-cli"},
			},
		},
	}
}

// Identify 基于 User-Agent 和目标域名动态识别工具来源
// 未知工具返回 "unknown"
func (db *FingerprintDB) Identify(userAgent string, targetHost string) string {
	ua := strings.ToLower(userAgent)
	host := strings.ToLower(targetHost)

	for _, fp := range db.fingerprints {
		for _, pattern := range fp.Patterns {
			if strings.Contains(ua, pattern) || strings.Contains(host, pattern) {
				return fp.ToolName
			}
		}
	}

	return "unknown"
}

// IdentifyFromRequest 从 HTTP 请求中识别工具
func (db *FingerprintDB) IdentifyFromRequest(req *http.Request) string {
	ua := req.Header.Get("User-Agent")
	host := req.URL.Host
	if host == "" {
		host = req.Host
	}
	return db.Identify(ua, host)
}

// AddFingerprint 添加新指纹（支持运行时扩展）
func (db *FingerprintDB) AddFingerprint(toolName string, patterns []string) {
	db.fingerprints = append(db.fingerprints, Fingerprint{
		ToolName: toolName,
		Patterns: patterns,
	})
}

// ListTools 列出所有已注册的工具名称
func (db *FingerprintDB) ListTools() []string {
	tools := make([]string, 0, len(db.fingerprints))
	for _, fp := range db.fingerprints {
		tools = append(tools, fp.ToolName)
	}
	return tools
}
