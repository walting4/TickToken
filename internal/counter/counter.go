package counter

import (
	"encoding/json"
	"log"
	"strings"

	"github.com/pkoukk/tiktoken-go"
)

// TokenCount token 计数结果
type TokenCount struct {
	PromptTokens     int    // prompt token 总数
	CompletionTokens int    // completion token 数
	Source           string // "response" 或 "local_tokenizer"
	Tokenizer        string // 使用的 tokenizer 名称（如 "o200k_base"、"cl100k_base"、"fallback"）
	Model            string // 模型名称
	HasUsage         bool   // 响应是否包含 usage 字段
}

// Counter 动态 token 计数引擎
type Counter struct {
	tokenizers map[string]*tiktoken.Tiktoken
}

// NewCounter 创建计数引擎
func NewCounter() *Counter {
	c := &Counter{
		tokenizers: make(map[string]*tiktoken.Tiktoken),
	}

	// 初始化常用 tokenizer
	encodings := []string{"cl100k_base", "o200k_base"}
	for _, enc := range encodings {
		if tke, err := tiktoken.GetEncoding(enc); err == nil {
			c.tokenizers[enc] = tke
		} else {
			log.Printf("[Counter] 警告: 无法加载 tokenizer %s: %v", enc, err)
		}
	}

	return c
}

// CountFromResponse 优先从 API 响应中提取 token usage（精度 100%）
// 尝试多种已知的 JSON 字段路径
func (c *Counter) CountFromResponse(respBody []byte, model string) (*TokenCount, bool) {
	var raw map[string]interface{}
	if err := json.Unmarshal(respBody, &raw); err != nil {
		return nil, false
	}

	tc := &TokenCount{Model: model}

	// 策略1: OpenAI 风格 usage.prompt_tokens / usage.completion_tokens
	if usage, ok := raw["usage"].(map[string]interface{}); ok {
		tc.HasUsage = true
		if pt, ok := usage["prompt_tokens"].(float64); ok {
			tc.PromptTokens = int(pt)
		}
		if ct, ok := usage["completion_tokens"].(float64); ok {
			tc.CompletionTokens = int(ct)
		}
		// 也尝试 input_tokens / output_tokens（Anthropic 风格）
		if it, ok := usage["input_tokens"].(float64); ok && tc.PromptTokens == 0 {
			tc.PromptTokens = int(it)
		}
		if ot, ok := usage["output_tokens"].(float64); ok && tc.CompletionTokens == 0 {
			tc.CompletionTokens = int(ot)
		}
		if tc.PromptTokens > 0 || tc.CompletionTokens > 0 {
			tc.Source = "response"
			tc.Tokenizer = "none"
			return tc, true
		}
	}

	// 策略2: Gemini 风格 usageMetadata.promptTokenCount
	if um, ok := raw["usageMetadata"].(map[string]interface{}); ok {
		tc.HasUsage = true
		if pt, ok := um["promptTokenCount"].(float64); ok {
			tc.PromptTokens = int(pt)
		}
		if ct, ok := um["candidatesTokenCount"].(float64); ok {
			tc.CompletionTokens = int(ct)
		}
		if tc.PromptTokens > 0 || tc.CompletionTokens > 0 {
			tc.Source = "response"
			tc.Tokenizer = "none"
			return tc, true
		}
	}

	// 策略3: 顶层 prompt_tokens / completion_tokens
	if pt, ok := raw["prompt_tokens"].(float64); ok {
		tc.HasUsage = true
		tc.PromptTokens = int(pt)
	}
	if ct, ok := raw["completion_tokens"].(float64); ok {
		tc.HasUsage = true
		tc.CompletionTokens = int(ct)
	}
	if tc.PromptTokens > 0 || tc.CompletionTokens > 0 {
		tc.Source = "response"
		tc.Tokenizer = "none"
		return tc, true
	}

	return nil, false
}

// CountWithTokenizer 使用本地 tokenizer 估算 token 数（兜底策略）
func (c *Counter) CountWithTokenizer(reqBody []byte, respBody []byte, model string) *TokenCount {
	tc := &TokenCount{
		Model:  model,
		Source: "local_tokenizer",
	}

	encName := c.routeTokenizer(model)
	tc.Tokenizer = encName

	tke, ok := c.tokenizers[encName]
	if !ok {
		// 兜底到 cl100k_base
		tke, ok = c.tokenizers["cl100k_base"]
		if !ok {
			tc.Tokenizer = "unavailable"
			return tc
		}
		if encName != "fallback" {
			tc.Tokenizer = "fallback"
		}
	}

	// 估算 prompt token
	if len(reqBody) > 0 {
		var reqMap map[string]interface{}
		if err := json.Unmarshal(reqBody, &reqMap); err == nil {
			if messages, ok := reqMap["messages"].([]interface{}); ok {
				for _, msg := range messages {
					if m, ok := msg.(map[string]interface{}); ok {
						if content, ok := m["content"].(string); ok {
							tc.PromptTokens += len(tke.Encode(content, nil, nil))
						}
					}
				}
			} else if content, ok := reqMap["prompt"].(string); ok {
				tc.PromptTokens += len(tke.Encode(content, nil, nil))
			} else if contents, ok := reqMap["contents"].([]interface{}); ok {
				// Gemini 格式
				for _, part := range contents {
					if p, ok := part.(map[string]interface{}); ok {
						if parts, ok := p["parts"].([]interface{}); ok {
							for _, pp := range parts {
								if pm, ok := pp.(map[string]interface{}); ok {
									if text, ok := pm["text"].(string); ok {
										tc.PromptTokens += len(tke.Encode(text, nil, nil))
									}
								}
							}
						}
					}
				}
			}
		}
	}

	// 估算 completion token
	if len(respBody) > 0 {
		var respMap map[string]interface{}
		if err := json.Unmarshal(respBody, &respMap); err == nil {
			// OpenAI 格式
			if choices, ok := respMap["choices"].([]interface{}); ok {
				for _, ch := range choices {
					if choice, ok := ch.(map[string]interface{}); ok {
						if msg, ok := choice["message"].(map[string]interface{}); ok {
							if content, ok := msg["content"].(string); ok {
								tc.CompletionTokens += len(tke.Encode(content, nil, nil))
							}
						}
					}
				}
			}
			// Anthropic 格式
			if content, ok := respMap["content"].([]interface{}); ok {
				for _, block := range content {
					if b, ok := block.(map[string]interface{}); ok {
						if text, ok := b["text"].(string); ok {
							tc.CompletionTokens += len(tke.Encode(text, nil, nil))
						}
					}
				}
			}
		}
	}

	return tc
}

// routeTokenizer 基于模型名模式匹配选择 tokenizer
// 支持主流模型系列的精确路由
func (c *Counter) routeTokenizer(model string) string {
	model = strings.ToLower(model)
	switch {
	// OpenAI o200k 系列（GPT-4o, GPT-4.1, o1, o3 等）
	case strings.HasPrefix(model, "gpt-4o"),
		strings.HasPrefix(model, "gpt-4.1"),
		strings.HasPrefix(model, "o1"),
		strings.HasPrefix(model, "o3"),
		strings.HasPrefix(model, "o4"):
		return "o200k_base"
	// OpenAI cl100k 系列（GPT-4, GPT-3.5, text-embedding）
	case strings.HasPrefix(model, "gpt-4"),
		strings.HasPrefix(model, "gpt-3.5"),
		strings.HasPrefix(model, "text-embedding"):
		return "cl100k_base"
	// Anthropic Claude 系列（使用 cl100k_base 近似，精度损失约 3-5%）
	case strings.HasPrefix(model, "claude"):
		return "cl100k_base"
	// DeepSeek 系列
	case strings.HasPrefix(model, "deepseek"):
		return "cl100k_base"
	// Google Gemini 系列（使用 cl100k_base 近似）
	case strings.HasPrefix(model, "gemini"),
		strings.HasPrefix(model, "gemma"):
		return "cl100k_base"
	// Meta LLaMA 系列
	case strings.HasPrefix(model, "llama"),
		strings.HasPrefix(model, "meta-llama"):
		return "cl100k_base"
	// Mistral 系列
	case strings.HasPrefix(model, "mistral"),
		strings.HasPrefix(model, "mixtral"):
		return "cl100k_base"
	// Qwen 系列
	case strings.HasPrefix(model, "qwen"):
		return "cl100k_base"
	default:
		return "fallback"
	}
}

// Count 统一计数入口：先尝试响应 usage，再回退到本地 tokenizer
func (c *Counter) Count(reqBody []byte, respBody []byte, model string) *TokenCount {
	// 优先从响应提取
	if tc, ok := c.CountFromResponse(respBody, model); ok {
		return tc
	}
	// 兜底到本地 tokenizer
	return c.CountWithTokenizer(reqBody, respBody, model)
}

// CountWithVerification 执行计数并返回本地估算结果（用于双重校验）
// 即使响应包含 usage，也会同时执行本地 tokenizer 估算
// 返回值：最终计数结果（优先 response usage）和本地估算结果
func (c *Counter) CountWithVerification(reqBody []byte, respBody []byte, model string) (final *TokenCount, local *TokenCount) {
	// 始终执行本地估算（用于校验）
	local = c.CountWithTokenizer(reqBody, respBody, model)

	// 尝试从响应提取
	if tc, ok := c.CountFromResponse(respBody, model); ok {
		return tc, local
	}

	// 无 response usage，本地估算即为最终结果
	return local, local
}
