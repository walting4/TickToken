package adapters

import (
	"net/http/httptest"
	"testing"
)

// TestFingerprintDB_Identify 测试工具指纹识别
func TestFingerprintDB_Identify(t *testing.T) {
	db := NewFingerprintDB()

	tests := []struct {
		name     string
		ua       string
		host     string
		expected string
	}{
		{"VS Code", "vscode/1.85.0", "api.openai.com", "vscode"},
		{"Cursor", "cursor/0.42.0", "api.openai.com", "cursor"},
		{"JetBrains", "JetBrains-IDEA/2024.1", "api.openai.com", "jetbrains"},
		{"Windsurf", "windsurf/1.0", "codeium.com", "windsurf"},
		{"TRAE", "trae/1.0", "api.trae.ai", "trae"},
		{"WorkBuddy", "workbuddy/1.0", "api.workbuddy.ai", "workbuddy"},
		{"Cline", "cline/2.0", "api.anthropic.com", "cline"},
		{"Aider", "aider/0.60", "api.openai.com", "aider"},
		{"Claude Code", "claude-code/1.0", "api.anthropic.com", "claude-code"},
		{"Codex CLI", "codex/1.0", "api.openai.com", "codex-cli"},
		{"Continue", "continue-dev/0.9", "api.openai.com", "continue"},
		{"Copilot CLI", "github-copilot-cli/1.0", "api.github.com", "copilot-cli"},
		{"Unknown", "some-random-tool/1.0", "example.com", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := db.Identify(tt.ua, tt.host)
			if got != tt.expected {
				t.Errorf("Identify(%s, %s) = %s，期望 %s", tt.ua, tt.host, got, tt.expected)
			}
		})
	}
}

// TestFingerprintDB_IdentifyFromRequest 测试从请求中识别
func TestFingerprintDB_IdentifyFromRequest(t *testing.T) {
	db := NewFingerprintDB()
	req := httptest.NewRequest("POST", "https://api.openai.com/v1/chat/completions", nil)
	req.Header.Set("User-Agent", "vscode/1.85.0")

	tool := db.IdentifyFromRequest(req)
	if tool != "vscode" {
		t.Errorf("期望 vscode，得到 %s", tool)
	}
}

// TestFingerprintDB_AddFingerprint 测试动态添加指纹
func TestFingerprintDB_AddFingerprint(t *testing.T) {
	db := NewFingerprintDB()

	// 添加新工具
	db.AddFingerprint("new-tool", []string{"new-tool", "newtool"})

	// 验证可识别
	tool := db.Identify("new-tool/1.0", "example.com")
	if tool != "new-tool" {
		t.Errorf("期望 new-tool，得到 %s", tool)
	}

	// 验证出现在列表中
	tools := db.ListTools()
	found := false
	for _, t := range tools {
		if t == "new-tool" {
			found = true
			break
		}
	}
	if !found {
		t.Error("new-tool 应在工具列表中")
	}
}

// TestFingerprintDB_ListTools 测试工具列表
func TestFingerprintDB_ListTools(t *testing.T) {
	db := NewFingerprintDB()
	tools := db.ListTools()
	// 应至少有 12 个内置工具
	if len(tools) < 12 {
		t.Errorf("期望至少 12 个工具，得到 %d", len(tools))
	}
}

// TestFileWatcher_Creation 测试文件监控器创建
func TestFileWatcher_Creation(t *testing.T) {
	w, err := NewFileWatcher(0) // 0 应被替换为默认值
	if err != nil {
		t.Fatalf("创建监控器失败: %v", err)
	}
	if w == nil {
		t.Fatal("监控器不应为 nil")
	}
}

// TestFileWatcher_StartStop 测试启动和停止
func TestFileWatcher_StartStop(t *testing.T) {
	w, err := NewFileWatcher(1)
	if err != nil {
		t.Fatalf("创建监控器失败: %v", err)
	}

	if err := w.Start(); err != nil {
		t.Fatalf("启动失败: %v", err)
	}

	// 重复启动应失败
	if err := w.Start(); err == nil {
		t.Error("重复启动应返回错误")
	}

	w.Stop()

	// 再次停止不应 panic
	w.Stop()
}

// TestFileWatcher_Events 测试事件通道
func TestFileWatcher_Events(t *testing.T) {
	w, _ := NewFileWatcher(1)
	ch := w.Events()
	if ch == nil {
		t.Fatal("事件通道不应为 nil")
	}
}
