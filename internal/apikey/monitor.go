// Package apikey 提供 API Key 调用的全生命周期监控。
// 它在 MITM 代理捕获的请求基础上，专门识别携带 API Key 的请求，
// 跟踪每个请求从发起到响应完成的完整生命周期，并精确记录 token 消耗。
package apikey

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"
)

// RequestState 请求生命周期状态
type RequestState int

const (
	StatePending   RequestState = iota // 已拦截，等待响应
	StateCompleted                     // 响应已到达，token 已计算
	StateFailed                        // 请求失败（网络错误/上游 4xx/5xx）
	StateAnomaly                       // token 计数异常
)

// String 返回状态的可读字符串
func (s RequestState) String() string {
	switch s {
	case StatePending:
		return "pending"
	case StateCompleted:
		return "completed"
	case StateFailed:
		return "failed"
	case StateAnomaly:
		return "anomaly"
	default:
		return "unknown"
	}
}

// APIKeyDetection 识别到的 API Key 信息
type APIKeyDetection struct {
	Present   bool   // 是否携带 API Key
	Provider  string // 推测的 provider：openai / anthropic / gemini / deepseek / generic
	KeyPrefix string // API Key 前缀（仅前 8 字符，用于脱敏标识，不记录完整 key）
	Hash      string // API Key 的 SHA256 哈希前 16 字符（用于去重与关联，不可逆）
	Header    string // 携带 key 的头部名称
}

// LifecycleEvent 单个请求的完整生命周期记录
type LifecycleEvent struct {
	RequestID    string        // 唯一请求 ID
	StartTime    time.Time     // 请求拦截时间
	EndTime      time.Time     // 响应处理完成时间
	Latency      time.Duration // 端到端延迟
	State        RequestState  // 最终状态
	Host         string        // 目标主机
	Method       string        // HTTP 方法
	Path         string        // 请求路径
	StatusCode   int           // HTTP 状态码
	Tool         string        // 来源工具（指纹识别）
	Model        string        // 模型名
	Provider     string        // provider
	KeyHash      string        // API Key 哈希
	PromptTokens int           // prompt token
	CompletionTokens int       // completion token
	CacheHit     int           // 缓存命中
	CacheMiss    int           // 缓存未命中
	Source       string        // 计数来源
	ErrorMsg     string        // 错误信息（StateFailed 时）
}

// Monitor API Key 调用监控器
// 它维护一个 inflight 请求表，在请求拦截时创建条目，
// 在响应处理时完成条目并计算延迟。线程安全。
type Monitor struct {
	mu       sync.RWMutex
	inflight map[string]*LifecycleEvent // requestID -> event
	stats    MonitorStats
}

// MonitorStats 监控统计
type MonitorStats struct {
	TotalRequests    int64 // 拦截到的 API Key 请求总数
	CompletedRequests int64 // 成功完成的请求数
	FailedRequests   int64 // 失败的请求数
	AnomalyRequests  int64 // 异常请求数
	TotalLatencyMs   int64 // 累计延迟（毫秒）
}

// NewMonitor 创建监控器
func NewMonitor() *Monitor {
	return &Monitor{
		inflight: make(map[string]*LifecycleEvent),
	}
}

// DetectAPIKey 从 HTTP 请求头中检测 API Key
// 支持 OpenAI/Anthropic/Gemini/DeepSeek 等主流 provider 的鉴权头格式
func DetectAPIKey(req *http.Request) APIKeyDetection {
	d := APIKeyDetection{}

	// Authorization: Bearer sk-xxx（OpenAI/DeepSeek/通用）
	if auth := req.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		key := strings.TrimPrefix(auth, "Bearer ")
		key = strings.TrimSpace(key)
		if len(key) >= 8 {
			d.Present = true
			d.Header = "Authorization"
			d.KeyPrefix = key[:min(8, len(key))]
			d.Hash = hashKey(key)
			d.Provider = classifyProvider(key, req.Host)
			return d
		}
	}

	// x-api-key: sk-ant-xxx（Anthropic）
	if key := req.Header.Get("x-api-key"); key != "" {
		key = strings.TrimSpace(key)
		if len(key) >= 8 {
			d.Present = true
			d.Header = "x-api-key"
			d.KeyPrefix = key[:min(8, len(key))]
			d.Hash = hashKey(key)
			d.Provider = classifyProvider(key, req.Host)
			return d
		}
	}

	// x-goog-api-key: xxx（Gemini）
	if key := req.Header.Get("x-goog-api-key"); key != "" {
		key = strings.TrimSpace(key)
		if len(key) >= 8 {
			d.Present = true
			d.Header = "x-goog-api-key"
			d.KeyPrefix = key[:min(8, len(key))]
			d.Hash = hashKey(key)
			d.Provider = "gemini"
			return d
		}
	}

	return d
}

// classifyProvider 根据 key 前缀和 host 推测 provider
func classifyProvider(key, host string) string {
	host = strings.ToLower(host)
	switch {
	case strings.Contains(host, "anthropic") || strings.Contains(host, "claude"):
		return "anthropic"
	case strings.Contains(host, "openai"):
		return "openai"
	case strings.Contains(host, "googleapis") || strings.Contains(host, "gemini"):
		return "gemini"
	case strings.Contains(host, "deepseek"):
		return "deepseek"
	case strings.HasPrefix(key, "sk-ant-"):
		return "anthropic"
	case strings.HasPrefix(key, "sk-"):
		return "openai"
	case strings.HasPrefix(key, "AIza"):
		return "gemini"
	default:
		return "generic"
	}
}

// DetectProviderByHost 仅根据 host 推测 provider（无 API Key 时使用）
func DetectProviderByHost(host string) string {
	return classifyProvider("", host)
}

// hashKey 计算 API Key 的哈希（不可逆，用于关联与去重）
func hashKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])[:16]
}

// StartRequest 在请求拦截时调用，创建 inflight 条目
// 返回 requestID 用于后续关联响应
func (m *Monitor) StartRequest(host string, req *http.Request, tool string) string {
	detection := DetectAPIKey(req)
	if !detection.Present {
		return ""
	}

	requestID := generateRequestID(host, req, detection.Hash)

	event := &LifecycleEvent{
		RequestID: requestID,
		StartTime: time.Now(),
		State:     StatePending,
		Host:      host,
		Method:    req.Method,
		Path:      req.URL.Path,
		Tool:      tool,
		Provider:  detection.Provider,
		KeyHash:   detection.Hash,
	}

	m.mu.Lock()
	m.inflight[requestID] = event
	m.stats.TotalRequests++
	m.mu.Unlock()

	return requestID
}

// CompleteRequest 在响应处理完成时调用，完成生命周期记录
// 返回最终的 LifecycleEvent（调用方可用于存储或推送）
func (m *Monitor) CompleteRequest(requestID string, statusCode int, model string,
	promptTokens, completionTokens, cacheHit, cacheMiss int, source string) *LifecycleEvent {

	if requestID == "" {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	event, ok := m.inflight[requestID]
	if !ok {
		return nil
	}
	delete(m.inflight, requestID)

	event.EndTime = time.Now()
	event.Latency = event.EndTime.Sub(event.StartTime)
	event.StatusCode = statusCode
	event.Model = model
	event.PromptTokens = promptTokens
	event.CompletionTokens = completionTokens
	event.CacheHit = cacheHit
	event.CacheMiss = cacheMiss
	event.Source = source

	// 判断状态
	if statusCode >= 400 {
		event.State = StateFailed
		m.stats.FailedRequests++
	} else {
		event.State = StateCompleted
		m.stats.CompletedRequests++
	}

	m.stats.TotalLatencyMs += event.Latency.Milliseconds()

	return event
}

// MarkFailed 标记请求失败（网络错误等，无响应）
func (m *Monitor) MarkFailed(requestID string, errMsg string) {
	if requestID == "" {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	event, ok := m.inflight[requestID]
	if !ok {
		return
	}
	delete(m.inflight, requestID)

	event.EndTime = time.Now()
	event.Latency = event.EndTime.Sub(event.StartTime)
	event.State = StateFailed
	event.ErrorMsg = errMsg
	m.stats.FailedRequests++
	m.stats.TotalLatencyMs += event.Latency.Milliseconds()
}

// MarkAnomaly 标记请求异常（token 计数可疑）
func (m *Monitor) MarkAnomaly(requestID string, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if event, ok := m.inflight[requestID]; ok {
		event.State = StateAnomaly
		event.ErrorMsg = reason
	}
	m.stats.AnomalyRequests++
}

// GetStats 获取监控统计快照
func (m *Monitor) GetStats() MonitorStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

// InflightCount 当前 inflight 请求数
func (m *Monitor) InflightCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.inflight)
}

// generateRequestID 生成唯一请求 ID
func generateRequestID(host string, req *http.Request, keyHash string) string {
	raw := host + req.Method + req.URL.Path + keyHash + time.Now().Format(time.RFC3339Nano)
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])[:16]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
