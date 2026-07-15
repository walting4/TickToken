package apikey

import (
	"net/http/httptest"
	"testing"
)

// TestDetectAPIKey_OpenAI 测试 OpenAI Bearer token 检测
func TestDetectAPIKey_OpenAI(t *testing.T) {
	req := httptest.NewRequest("POST", "https://api.openai.com/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer sk-proj-1234567890abcdef")

	d := DetectAPIKey(req)
	if !d.Present {
		t.Fatal("应检测到 API Key")
	}
	if d.Header != "Authorization" {
		t.Errorf("Header 期望 Authorization，得到 %s", d.Header)
	}
	if d.Provider != "openai" {
		t.Errorf("Provider 期望 openai，得到 %s", d.Provider)
	}
	if d.KeyPrefix != "sk-proj-" {
		t.Errorf("KeyPrefix 期望 sk-proj-，得到 %s", d.KeyPrefix)
	}
	if d.Hash == "" {
		t.Error("Hash 不应为空")
	}
}

// TestDetectAPIKey_Anthropic 测试 Anthropic x-api-key 检测
func TestDetectAPIKey_Anthropic(t *testing.T) {
	req := httptest.NewRequest("POST", "https://api.anthropic.com/v1/messages", nil)
	req.Header.Set("x-api-key", "sk-ant-api03-1234567890")

	d := DetectAPIKey(req)
	if !d.Present {
		t.Fatal("应检测到 API Key")
	}
	if d.Header != "x-api-key" {
		t.Errorf("Header 期望 x-api-key，得到 %s", d.Header)
	}
	if d.Provider != "anthropic" {
		t.Errorf("Provider 期望 anthropic，得到 %s", d.Provider)
	}
}

// TestDetectAPIKey_Gemini 测试 Gemini x-goog-api-key 检测
func TestDetectAPIKey_Gemini(t *testing.T) {
	req := httptest.NewRequest("POST", "https://generativelanguage.googleapis.com/v1beta/models/gemini-pro:generateContent", nil)
	req.Header.Set("x-goog-api-key", "AIzaSyD-1234567890abcdef")

	d := DetectAPIKey(req)
	if !d.Present {
		t.Fatal("应检测到 API Key")
	}
	if d.Header != "x-goog-api-key" {
		t.Errorf("Header 期望 x-goog-api-key，得到 %s", d.Header)
	}
	if d.Provider != "gemini" {
		t.Errorf("Provider 期望 gemini，得到 %s", d.Provider)
	}
}

// TestDetectAPIKey_NoKey 测试无 API Key 的请求
func TestDetectAPIKey_NoKey(t *testing.T) {
	req := httptest.NewRequest("POST", "https://api.openai.com/v1/chat/completions", nil)

	d := DetectAPIKey(req)
	if d.Present {
		t.Fatal("不应检测到 API Key")
	}
}

// TestDetectAPIKey_ShortKey 测试过短的 key（应忽略）
func TestDetectAPIKey_ShortKey(t *testing.T) {
	req := httptest.NewRequest("POST", "https://api.openai.com/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer short")

	d := DetectAPIKey(req)
	if d.Present {
		t.Fatal("过短的 key 不应被检测")
	}
}

// TestMonitor_Lifecycle 测试完整生命周期跟踪
func TestMonitor_Lifecycle(t *testing.T) {
	m := NewMonitor()
	req := httptest.NewRequest("POST", "https://api.openai.com/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer sk-proj-test1234567890")

	// 启动请求
	requestID := m.StartRequest("api.openai.com", req, "vscode")
	if requestID == "" {
		t.Fatal("应返回有效的 requestID")
	}

	// inflight 应为 1
	if count := m.InflightCount(); count != 1 {
		t.Errorf("InflightCount 期望 1，得到 %d", count)
	}

	// 完成请求
	event := m.CompleteRequest(requestID, 200, "gpt-4o", 100, 50, 80, 20, "response")
	if event == nil {
		t.Fatal("应返回 lifecycle event")
	}
	if event.State != StateCompleted {
		t.Errorf("State 期望 completed，得到 %s", event.State)
	}
	if event.StatusCode != 200 {
		t.Errorf("StatusCode 期望 200，得到 %d", event.StatusCode)
	}
	if event.PromptTokens != 100 {
		t.Errorf("PromptTokens 期望 100，得到 %d", event.PromptTokens)
	}

	// inflight 应为 0
	if count := m.InflightCount(); count != 0 {
		t.Errorf("InflightCount 期望 0，得到 %d", count)
	}

	// 统计应正确
	stats := m.GetStats()
	if stats.TotalRequests != 1 {
		t.Errorf("TotalRequests 期望 1，得到 %d", stats.TotalRequests)
	}
	if stats.CompletedRequests != 1 {
		t.Errorf("CompletedRequests 期望 1，得到 %d", stats.CompletedRequests)
	}
}

// TestMonitor_FailedRequest 测试失败请求标记
func TestMonitor_FailedRequest(t *testing.T) {
	m := NewMonitor()
	req := httptest.NewRequest("POST", "https://api.openai.com/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer sk-proj-test1234567890")

	requestID := m.StartRequest("api.openai.com", req, "cursor")

	// 标记为失败
	m.MarkFailed(requestID, "connection refused")

	stats := m.GetStats()
	if stats.FailedRequests != 1 {
		t.Errorf("FailedRequests 期望 1，得到 %d", stats.FailedRequests)
	}
}

// TestMonitor_AnomalyRequest 测试异常标记
func TestMonitor_AnomalyRequest(t *testing.T) {
	m := NewMonitor()
	req := httptest.NewRequest("POST", "https://api.openai.com/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer sk-proj-test1234567890")

	requestID := m.StartRequest("api.openai.com", req, "trae")

	// 标记为异常
	m.MarkAnomaly(requestID, "token deviation 50%")

	stats := m.GetStats()
	if stats.AnomalyRequests != 1 {
		t.Errorf("AnomalyRequests 期望 1，得到 %d", stats.AnomalyRequests)
	}
}

// TestMonitor_HttpErrorStatus 测试 HTTP 4xx/5xx 被标记为失败
func TestMonitor_HttpErrorStatus(t *testing.T) {
	m := NewMonitor()
	req := httptest.NewRequest("POST", "https://api.openai.com/v1/chat/completions", nil)
	req.Header.Set("Authorization", "Bearer sk-proj-test1234567890")

	requestID := m.StartRequest("api.openai.com", req, "vscode")

	event := m.CompleteRequest(requestID, 429, "gpt-4o", 0, 0, 0, 0, "response")
	if event == nil {
		t.Fatal("应返回 event")
	}
	if event.State != StateFailed {
		t.Errorf("429 状态应标记为 failed，得到 %s", event.State)
	}
}

// TestMonitor_NonAPIKeyRequest 测试非 API Key 请求被忽略
func TestMonitor_NonAPIKeyRequest(t *testing.T) {
	m := NewMonitor()
	req := httptest.NewRequest("POST", "https://api.openai.com/v1/chat/completions", nil)
	// 不设置 API Key header

	requestID := m.StartRequest("api.openai.com", req, "vscode")
	if requestID != "" {
		t.Fatal("非 API Key 请求应返回空 requestID")
	}
}

// TestMonitor_Concurrent 测试并发安全性
func TestMonitor_Concurrent(t *testing.T) {
	m := NewMonitor()

	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func() {
			req := httptest.NewRequest("POST", "https://api.openai.com/v1/chat/completions", nil)
			req.Header.Set("Authorization", "Bearer sk-proj-test1234567890")
			rid := m.StartRequest("api.openai.com", req, "vscode")
			if rid != "" {
				m.CompleteRequest(rid, 200, "gpt-4o", 10, 5, 0, 10, "response")
			}
			done <- true
		}()
	}

	for i := 0; i < 100; i++ {
		<-done
	}

	stats := m.GetStats()
	if stats.TotalRequests != 100 {
		t.Errorf("TotalRequests 期望 100，得到 %d", stats.TotalRequests)
	}
	if stats.CompletedRequests != 100 {
		t.Errorf("CompletedRequests 期望 100，得到 %d", stats.CompletedRequests)
	}
}

// TestDetectProviderByHost 测试基于 host 的 provider 推断
func TestDetectProviderByHost(t *testing.T) {
	tests := []struct {
		host     string
		expected string
	}{
		{"api.openai.com", "openai"},
		{"api.anthropic.com", "anthropic"},
		{"generativelanguage.googleapis.com", "gemini"},
		{"api.deepseek.com", "deepseek"},
		{"unknown.example.com", "generic"},
	}

	for _, tt := range tests {
		got := DetectProviderByHost(tt.host)
		if got != tt.expected {
			t.Errorf("DetectProviderByHost(%s) = %s，期望 %s", tt.host, got, tt.expected)
		}
	}
}
