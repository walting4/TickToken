package counter

import (
	"encoding/json"
	"testing"
)

// TestCountFromResponse_OpenAI 测试 OpenAI 风格 usage 提取
func TestCountFromResponse_OpenAI(t *testing.T) {
	c := NewCounter()
	respBody := []byte(`{
		"usage": {
			"prompt_tokens": 100,
			"completion_tokens": 50,
			"total_tokens": 150
		}
	}`)

	tc, ok := c.CountFromResponse(respBody, "gpt-4o")
	if !ok {
		t.Fatal("应成功提取 usage")
	}
	if tc.PromptTokens != 100 {
		t.Errorf("PromptTokens 期望 100，得到 %d", tc.PromptTokens)
	}
	if tc.CompletionTokens != 50 {
		t.Errorf("CompletionTokens 期望 50，得到 %d", tc.CompletionTokens)
	}
	if tc.Source != "response" {
		t.Errorf("Source 期望 response，得到 %s", tc.Source)
	}
	if !tc.HasUsage {
		t.Error("HasUsage 应为 true")
	}
}

// TestCountFromResponse_Anthropic 测试 Anthropic 风格 usage 提取
func TestCountFromResponse_Anthropic(t *testing.T) {
	c := NewCounter()
	respBody := []byte(`{
		"usage": {
			"input_tokens": 200,
			"output_tokens": 80
		}
	}`)

	tc, ok := c.CountFromResponse(respBody, "claude-3-5-sonnet-20241022")
	if !ok {
		t.Fatal("应成功提取 usage")
	}
	if tc.PromptTokens != 200 {
		t.Errorf("PromptTokens 期望 200，得到 %d", tc.PromptTokens)
	}
	if tc.CompletionTokens != 80 {
		t.Errorf("CompletionTokens 期望 80，得到 %d", tc.CompletionTokens)
	}
}

// TestCountFromResponse_Gemini 测试 Gemini 风格 usageMetadata 提取
func TestCountFromResponse_Gemini(t *testing.T) {
	c := NewCounter()
	respBody := []byte(`{
		"usageMetadata": {
			"promptTokenCount": 300,
			"candidatesTokenCount": 120
		}
	}`)

	tc, ok := c.CountFromResponse(respBody, "gemini-1.5-pro")
	if !ok {
		t.Fatal("应成功提取 usageMetadata")
	}
	if tc.PromptTokens != 300 {
		t.Errorf("PromptTokens 期望 300，得到 %d", tc.PromptTokens)
	}
	if tc.CompletionTokens != 120 {
		t.Errorf("CompletionTokens 期望 120，得到 %d", tc.CompletionTokens)
	}
}

// TestCountFromResponse_NoUsage 测试无 usage 的响应
func TestCountFromResponse_NoUsage(t *testing.T) {
	c := NewCounter()
	respBody := []byte(`{"choices": [{"message": {"content": "hello"}}]}`)

	tc, ok := c.CountFromResponse(respBody, "gpt-4o")
	if ok {
		t.Fatal("无 usage 时应返回 false")
	}
	if tc != nil {
		t.Error("无 usage 时 tc 应为 nil")
	}
}

// TestCountFromResponse_InvalidJSON 测试无效 JSON
func TestCountFromResponse_InvalidJSON(t *testing.T) {
	c := NewCounter()
	respBody := []byte(`{invalid json`)

	_, ok := c.CountFromResponse(respBody, "gpt-4o")
	if ok {
		t.Fatal("无效 JSON 应返回 false")
	}
}

// TestCountWithTokenizer_OpenAI 测试本地 tokenizer 估算 OpenAI 格式
func TestCountWithTokenizer_OpenAI(t *testing.T) {
	c := NewCounter()
	reqBody := []byte(`{
		"model": "gpt-4o",
		"messages": [
			{"role": "user", "content": "Hello, how are you?"}
		]
	}`)
	respBody := []byte(`{
		"choices": [
			{"message": {"content": "I'm doing well, thank you!"}}
		]
	}`)

	tc := c.CountWithTokenizer(reqBody, respBody, "gpt-4o")
	if tc.Source != "local_tokenizer" {
		t.Errorf("Source 期望 local_tokenizer，得到 %s", tc.Source)
	}
	if tc.PromptTokens <= 0 {
		t.Errorf("PromptTokens 应 > 0，得到 %d", tc.PromptTokens)
	}
	if tc.CompletionTokens <= 0 {
		t.Errorf("CompletionTokens 应 > 0，得到 %d", tc.CompletionTokens)
	}
	if tc.Tokenizer != "o200k_base" {
		t.Errorf("Tokenizer 期望 o200k_base，得到 %s", tc.Tokenizer)
	}
}

// TestRouteTokenizer 测试模型到 tokenizer 的路由
func TestRouteTokenizer(t *testing.T) {
	c := NewCounter()
	tests := []struct {
		model    string
		expected string
	}{
		{"gpt-4o", "o200k_base"},
		{"gpt-4o-mini", "o200k_base"},
		{"gpt-4.1", "o200k_base"},
		{"o1-preview", "o200k_base"},
		{"o3-mini", "o200k_base"},
		{"gpt-4", "cl100k_base"},
		{"gpt-4-turbo", "cl100k_base"},
		{"gpt-3.5-turbo", "cl100k_base"},
		{"claude-3-5-sonnet", "cl100k_base"},
		{"deepseek-chat", "cl100k_base"},
		{"gemini-1.5-pro", "cl100k_base"},
		{"llama-3-70b", "cl100k_base"},
		{"mistral-large", "cl100k_base"},
		{"qwen-2.5", "cl100k_base"},
		{"text-embedding-ada-002", "cl100k_base"},
		{"unknown-model", "fallback"},
	}

	for _, tt := range tests {
		got := c.routeTokenizer(tt.model)
		if got != tt.expected {
			t.Errorf("routeTokenizer(%s) = %s，期望 %s", tt.model, got, tt.expected)
		}
	}
}

// TestCount_ResponsePriority 测试 Count 优先使用 response usage
func TestCount_ResponsePriority(t *testing.T) {
	c := NewCounter()
	reqBody := []byte(`{"messages": [{"content": "hello"}]}`)
	respBody := []byte(`{"usage": {"prompt_tokens": 100, "completion_tokens": 50}}`)

	tc := c.Count(reqBody, respBody, "gpt-4o")
	if tc.Source != "response" {
		t.Errorf("应优先使用 response usage，得到 Source=%s", tc.Source)
	}
	if tc.PromptTokens != 100 {
		t.Errorf("PromptTokens 期望 100，得到 %d", tc.PromptTokens)
	}
}

// TestCount_FallbackToLocal 测试无 usage 时回退到本地 tokenizer
func TestCount_FallbackToLocal(t *testing.T) {
	c := NewCounter()
	reqBody := []byte(`{"messages": [{"content": "hello"}]}`)
	respBody := []byte(`{"choices": [{"message": {"content": "hi"}}]}`)

	tc := c.Count(reqBody, respBody, "gpt-4o")
	if tc.Source != "local_tokenizer" {
		t.Errorf("无 usage 时应回退到 local_tokenizer，得到 Source=%s", tc.Source)
	}
}

// TestCountWithVerification 测试双重校验计数
func TestCountWithVerification(t *testing.T) {
	c := NewCounter()
	reqBody := []byte(`{"messages": [{"content": "Hello world"}]}`)
	respBody := []byte(`{"usage": {"prompt_tokens": 10, "completion_tokens": 5}, "choices": [{"message": {"content": "Hi"}}]}`)

	final, local := c.CountWithVerification(reqBody, respBody, "gpt-4o")

	if final.Source != "response" {
		t.Errorf("final Source 期望 response，得到 %s", final.Source)
	}
	if final.PromptTokens != 10 {
		t.Errorf("final PromptTokens 期望 10，得到 %d", final.PromptTokens)
	}
	if local.Source != "local_tokenizer" {
		t.Errorf("local Source 期望 local_tokenizer，得到 %s", local.Source)
	}
}

// TestCountWithVerification_NoUsage 测试无 usage 时的双重校验
func TestCountWithVerification_NoUsage(t *testing.T) {
	c := NewCounter()
	reqBody := []byte(`{"messages": [{"content": "Hello world"}]}`)
	respBody := []byte(`{"choices": [{"message": {"content": "Hi there"}}]}`)

	final, local := c.CountWithVerification(reqBody, respBody, "gpt-4o")

	// 无 usage 时 final 和 local 应相同
	if final != local {
		t.Error("无 usage 时 final 和 local 应为同一对象")
	}
	if final.Source != "local_tokenizer" {
		t.Errorf("Source 期望 local_tokenizer，得到 %s", final.Source)
	}
}

// TestCountWithTokenizer_GeminiFormat 测试 Gemini 格式的本地估算
func TestCountWithTokenizer_GeminiFormat(t *testing.T) {
	c := NewCounter()
	reqBody := []byte(`{
		"contents": [
			{"parts": [{"text": "Hello, how are you?"}]}
		]
	}`)
	respBody := []byte(`{}`)

	tc := c.CountWithTokenizer(reqBody, respBody, "gemini-1.5-pro")
	if tc.PromptTokens <= 0 {
		t.Errorf("Gemini 格式 PromptTokens 应 > 0，得到 %d", tc.PromptTokens)
	}
}

// TestExtractModelFromRequest 测试模型名提取
func TestExtractModelFromRequest(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		want    string
	}{
		{"正常", `{"model": "gpt-4o", "messages": []}`, "gpt-4o"},
		{"无 model 字段", `{"messages": []}`, ""},
		{"空 body", ``, ""},
		{"无效 JSON", `{invalid}`, ""},
		{"model 非 string", `{"model": 123}`, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractModelFromRequest([]byte(tt.body))
			if got != tt.want {
				t.Errorf("ExtractModelFromRequest() = %s，期望 %s", got, tt.want)
			}
		})
	}
}

// TestCountWithTokenizer_AnthropicResponse 测试 Anthropic 格式响应的本地估算
func TestCountWithTokenizer_AnthropicResponse(t *testing.T) {
	c := NewCounter()
	reqBody := []byte(`{"messages": [{"content": "Hello"}]}`)
	respBody := []byte(`{
		"content": [
			{"type": "text", "text": "Hi there, how can I help you?"}
		]
	}`)

	tc := c.CountWithTokenizer(reqBody, respBody, "claude-3-5-sonnet")
	if tc.CompletionTokens <= 0 {
		t.Errorf("Anthropic 格式 CompletionTokens 应 > 0，得到 %d", tc.CompletionTokens)
	}
}

// TestCountFromResponse_DeepSeekCache 测试 DeepSeek 缓存字段
func TestCountFromResponse_DeepSeekCache(t *testing.T) {
	c := NewCounter()
	respBody := []byte(`{
		"usage": {
			"prompt_tokens": 100,
			"completion_tokens": 50,
			"prompt_cache_hit_tokens": 60,
			"prompt_cache_miss_tokens": 40
		}
	}`)

	tc, ok := c.CountFromResponse(respBody, "deepseek-chat")
	if !ok {
		t.Fatal("应成功提取 usage")
	}
	if tc.PromptTokens != 100 {
		t.Errorf("PromptTokens 期望 100，得到 %d", tc.PromptTokens)
	}
	if tc.CompletionTokens != 50 {
		t.Errorf("CompletionTokens 期望 50，得到 %d", tc.CompletionTokens)
	}
}

// BenchmarkCountFromResponse 性能测试：响应 usage 提取
func BenchmarkCountFromResponse(b *testing.B) {
	c := NewCounter()
	respBody := []byte(`{"usage": {"prompt_tokens": 100, "completion_tokens": 50}}`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.CountFromResponse(respBody, "gpt-4o")
	}
}

// BenchmarkCountWithTokenizer 性能测试：本地 tokenizer 估算
func BenchmarkCountWithTokenizer(b *testing.B) {
	c := NewCounter()
	reqBody, _ := json.Marshal(map[string]interface{}{
		"messages": []map[string]string{{"role": "user", "content": "Hello, how are you today?"}},
	})
	respBody := []byte(`{"choices": [{"message": {"content": "I'm doing well!"}}]}`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.CountWithTokenizer(reqBody, respBody, "gpt-4o")
	}
}
