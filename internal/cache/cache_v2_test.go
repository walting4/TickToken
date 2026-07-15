package cache

import "testing"

// TestParseFromResponse_Anthropic 测试 Anthropic 缓存字段
func TestParseFromResponse_Anthropic(t *testing.T) {
	respBody := []byte(`{
		"usage": {
			"cache_creation_input_tokens": 100,
			"cache_read_input_tokens": 200,
			"input_tokens": 50,
			"output_tokens": 30
		}
	}`)

	info := ParseFromResponse(respBody, 350)
	if info.CacheCreation != 100 {
		t.Errorf("CacheCreation 期望 100，得到 %d", info.CacheCreation)
	}
	if info.CacheHit != 200 {
		t.Errorf("CacheHit 期望 200，得到 %d", info.CacheHit)
	}
	if info.CacheStatus != "hit" {
		t.Errorf("CacheStatus 期望 hit，得到 %s", info.CacheStatus)
	}
}

// TestParseFromResponse_OpenAI 测试 OpenAI 缓存字段
func TestParseFromResponse_OpenAI(t *testing.T) {
	respBody := []byte(`{
		"usage": {
			"prompt_tokens": 100,
			"completion_tokens": 50,
			"prompt_tokens_details": {
				"cached_tokens": 60
			}
		}
	}`)

	info := ParseFromResponse(respBody, 100)
	if info.CacheHit != 60 {
		t.Errorf("CacheHit 期望 60，得到 %d", info.CacheHit)
	}
	if info.CacheStatus != "hit" {
		t.Errorf("CacheStatus 期望 hit，得到 %s", info.CacheStatus)
	}
}

// TestParseFromResponse_DeepSeek 测试 DeepSeek 缓存字段
func TestParseFromResponse_DeepSeek(t *testing.T) {
	respBody := []byte(`{
		"usage": {
			"prompt_tokens": 100,
			"completion_tokens": 50,
			"prompt_cache_hit_tokens": 70,
			"prompt_cache_miss_tokens": 30
		}
	}`)

	info := ParseFromResponse(respBody, 100)
	if info.CacheHit != 70 {
		t.Errorf("CacheHit 期望 70，得到 %d", info.CacheHit)
	}
	if info.CacheMiss != 30 {
		t.Errorf("CacheMiss 期望 30，得到 %d", info.CacheMiss)
	}
}

// TestParseFromResponse_Gemini 测试 Gemini 缓存字段
func TestParseFromResponse_Gemini(t *testing.T) {
	respBody := []byte(`{
		"usageMetadata": {
			"promptTokenCount": 100,
			"candidatesTokenCount": 50,
			"cachedContentTokenCount": 80
		}
	}`)

	info := ParseFromResponse(respBody, 100)
	if info.CacheHit != 80 {
		t.Errorf("CacheHit 期望 80，得到 %d", info.CacheHit)
	}
}

// TestParseFromResponse_NoCache 测试无缓存字段
func TestParseFromResponse_NoCache(t *testing.T) {
	respBody := []byte(`{
		"usage": {
			"prompt_tokens": 100,
			"completion_tokens": 50
		}
	}`)

	info := ParseFromResponse(respBody, 100)
	if info.CacheStatus != "unknown" {
		t.Errorf("CacheStatus 期望 unknown，得到 %s", info.CacheStatus)
	}
	if info.CacheMiss != 100 {
		t.Errorf("CacheMiss 期望 100，得到 %d", info.CacheMiss)
	}
}

// TestParseFromResponse_InvalidJSON 测试无效 JSON
func TestParseFromResponse_InvalidJSON(t *testing.T) {
	respBody := []byte(`{invalid`)

	info := ParseFromResponse(respBody, 100)
	if info.CacheStatus != "unknown" {
		t.Errorf("CacheStatus 期望 unknown，得到 %s", info.CacheStatus)
	}
	if info.CacheMiss != 100 {
		t.Errorf("CacheMiss 期望 100，得到 %d", info.CacheMiss)
	}
}

// TestParseFromResponse_CacheMissCalculation 测试缓存未命中计算
func TestParseFromResponse_CacheMissCalculation(t *testing.T) {
	respBody := []byte(`{
		"usage": {
			"cache_read_input_tokens": 80,
			"input_tokens": 100
		}
	}`)

	info := ParseFromResponse(respBody, 100)
	// cacheMiss = promptTokens - cacheHit - cacheCreation = 100 - 80 - 0 = 20
	if info.CacheMiss != 20 {
		t.Errorf("CacheMiss 期望 20，得到 %d", info.CacheMiss)
	}
}
