package counter

import "encoding/json"

// ExtractModelFromRequest 从请求 payload 中动态提取模型名称
func ExtractModelFromRequest(reqBody []byte) string {
	if len(reqBody) == 0 {
		return ""
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(reqBody, &raw); err != nil {
		return ""
	}

	// 尝试常见的 model 字段
	if model, ok := raw["model"].(string); ok {
		return model
	}

	return ""
}
