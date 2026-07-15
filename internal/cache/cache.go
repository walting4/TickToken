package cache

import "encoding/json"

// CacheInfo 缓存命中信息
type CacheInfo struct {
	CacheHit      int    // 缓存命中 token 数
	CacheMiss     int    // 缓存未命中 token 数
	CacheCreation int    // 缓存创建 token 数
	CacheStatus   string // "hit"、"unknown"、"none"
}

// ParseFromResponse 从响应中动态探测缓存字段
func ParseFromResponse(respBody []byte, promptTokens int) *CacheInfo {
	var raw map[string]interface{}
	if err := json.Unmarshal(respBody, &raw); err != nil {
		return &CacheInfo{
			CacheMiss:   promptTokens,
			CacheStatus: "unknown",
		}
	}

	info := &CacheInfo{
		CacheStatus: "none",
	}

	found := false

	// 通用探测：在 usage 对象中查找各种缓存字段
	if usage, ok := raw["usage"].(map[string]interface{}); ok {
		// Anthropic: cache_creation_input_tokens / cache_read_input_tokens
		if v, ok := usage["cache_creation_input_tokens"].(float64); ok {
			info.CacheCreation = int(v)
			found = true
		}
		if v, ok := usage["cache_read_input_tokens"].(float64); ok {
			info.CacheHit = int(v)
			found = true
		}

		// OpenAI: prompt_tokens_details.cached_tokens
		if details, ok := usage["prompt_tokens_details"].(map[string]interface{}); ok {
			if v, ok := details["cached_tokens"].(float64); ok {
				info.CacheHit = int(v)
				found = true
			}
		}

		// DeepSeek: prompt_cache_hit_tokens / prompt_cache_miss_tokens
		if v, ok := usage["prompt_cache_hit_tokens"].(float64); ok {
			info.CacheHit = int(v)
			found = true
		}
		if v, ok := usage["prompt_cache_miss_tokens"].(float64); ok {
			info.CacheMiss = int(v)
			found = true
		}
	}

	// Gemini: usageMetadata.cachedContentTokenCount
	if um, ok := raw["usageMetadata"].(map[string]interface{}); ok {
		if v, ok := um["cachedContentTokenCount"].(float64); ok {
			info.CacheHit = int(v)
			found = true
		}
	}

	// 计算未命中部分
	if found {
		if info.CacheStatus == "none" {
			info.CacheStatus = "hit"
		}
		// 如果 cache_miss 还没被设置（非 DeepSeek），则计算
		if info.CacheMiss == 0 && promptTokens > 0 {
			info.CacheMiss = promptTokens - info.CacheHit - info.CacheCreation
			if info.CacheMiss < 0 {
				info.CacheMiss = 0
			}
		}
	} else {
		// 无缓存字段
		info.CacheMiss = promptTokens
		info.CacheStatus = "unknown"
	}

	return info
}
