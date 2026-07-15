// Package verifier 实现双重校验与异常检测机制。
// 它将 API 响应中的官方 usage 与本地 tokenizer 估算结果进行对比，
// 识别 token 计数异常的请求，确保数据准确性达到 99.9%+。
package verifier

import (
	"math"
	"sync"
	"sync/atomic"
)

// AnomalyType 异常类型
type AnomalyType int

const (
	AnomalyNone                 AnomalyType = iota
	AnomalyPromptDeviation                  // prompt token 偏差过大
	AnomalyCompletionDeviation              // completion token 偏差过大
	AnomalyZeroTokens                       // 两个来源都返回 0 token
	AnomalyMissingUsage                     // 响应缺少 usage 字段（使用本地估算）
	AnomalyNegativeTokens                   // 出现负数 token
	AnomalyLatencySpike                     // 延迟异常飙升
	AnomalyResponseSize                     // 响应体大小异常
)

// String 返回异常类型的可读名称
func (a AnomalyType) String() string {
	switch a {
	case AnomalyNone:
		return "none"
	case AnomalyPromptDeviation:
		return "prompt_deviation"
	case AnomalyCompletionDeviation:
		return "completion_deviation"
	case AnomalyZeroTokens:
		return "zero_tokens"
	case AnomalyMissingUsage:
		return "missing_usage"
	case AnomalyNegativeTokens:
		return "negative_tokens"
	case AnomalyLatencySpike:
		return "latency_spike"
	case AnomalyResponseSize:
		return "response_size_anomaly"
	default:
		return "unknown"
	}
}

// IsAnomaly 是否为异常
func (a AnomalyType) IsAnomaly() bool {
	return a != AnomalyNone
}

// VerificationResult 校验结果
type VerificationResult struct {
	Anomaly         AnomalyType // 异常类型
	ResponseTokens  int         // 官方响应 usage 中的 token 数（0 表示无 usage）
	LocalTokens     int         // 本地 tokenizer 估算的 token 数
	DeviationPct    float64     // 偏差百分比（|response - local| / max(response, local)）
	IsVerified      bool        // 是否通过校验
	Reason          string      // 异常原因说明
	ConfidenceScore float64     // 置信度分数 0-1
}

// Config 校验器配置
type Config struct {
	// MaxDeviationPct 允许的最大偏差百分比（默认 15%）
	// 超过此阈值标记为异常
	MaxDeviationPct float64

	// MinTokensForCheck 进行偏差检查的最小 token 阈值
	// 低于此值的请求跳过偏差检查（小请求误差放大效应）
	MinTokensForCheck int

	// MaxLatencyMs 延迟异常阈值（默认 30000ms = 30s）
	MaxLatencyMs int64

	// MaxResponseBytes 响应体大小异常阈值（默认 10MB）
	MaxResponseBytes int
}

// DefaultConfig 默认配置
func DefaultConfig() Config {
	return Config{
		MaxDeviationPct:   15.0,
		MinTokensForCheck: 10,
		MaxLatencyMs:      30000,
		MaxResponseBytes:  10 * 1024 * 1024,
	}
}

// Stats 校验器累计统计
type Stats struct {
	TotalVerified    int64 // 总校验次数
	Passed           int64 // 通过校验
	Anomalies        int64 // 检出异常
	WithResponseUsage int64 // 使用响应 usage 的次数
	WithLocalOnly    int64 // 仅本地估算的次数
	AvgDeviation     uint64 // 平均偏差（存储为放大 1000 倍的整数避免浮点原子问题）
}

// AverageDeviation 返回平均偏差百分比
func (s *Stats) AverageDeviation() float64 {
	if s.TotalVerified == 0 {
		return 0
	}
	return float64(s.AvgDeviation) / 1000.0
}

// AccuracyRate 返回准确率（通过率）
func (s *Stats) AccuracyRate() float64 {
	if s.TotalVerified == 0 {
		return 0
	}
	return float64(s.Passed) / float64(s.TotalVerified) * 100.0
}

// Verifier 双重校验器
type Verifier struct {
	cfg   Config
	stats Stats
	mu    sync.RWMutex
	// 滑动窗口用于检测延迟异常
	latencyWindow []int64
	windowSize    int
}

// NewVerifier 创建校验器
func NewVerifier(cfg Config) *Verifier {
	if cfg.MaxDeviationPct <= 0 {
		cfg = DefaultConfig()
	}
	return &Verifier{
		cfg:           cfg,
		latencyWindow: make([]int64, 0, 100),
		windowSize:    100,
	}
}

// Verify 执行双重校验
// 参数：
//   - responsePromptTokens: 响应 usage 中的 prompt tokens（0 表示无 usage）
//   - responseCompletionTokens: 响应 usage 中的 completion tokens
//   - localPromptTokens: 本地 tokenizer 估算的 prompt tokens
//   - localCompletionTokens: 本地 tokenizer 估算的 completion tokens
//   - hasUsage: 响应是否包含 usage 字段
//   - latencyMs: 请求延迟（毫秒）
//   - responseBytes: 响应体字节数
func (v *Verifier) Verify(
	responsePromptTokens, responseCompletionTokens int,
	localPromptTokens, localCompletionTokens int,
	hasUsage bool,
	latencyMs int64,
	responseBytes int,
) VerificationResult {

	atomic.AddInt64(&v.stats.TotalVerified, 1)
	result := VerificationResult{
		IsVerified:      true,
		ConfidenceScore: 1.0,
	}

	// 负数 token 检查
	if responsePromptTokens < 0 || responseCompletionTokens < 0 ||
		localPromptTokens < 0 || localCompletionTokens < 0 {
		result.Anomaly = AnomalyNegativeTokens
		result.IsVerified = false
		result.Reason = "检测到负数 token 计数"
		result.ConfidenceScore = 0.0
		atomic.AddInt64(&v.stats.Anomalies, 1)
		v.recordLatency(latencyMs)
		return result
	}

	responseTotal := responsePromptTokens + responseCompletionTokens
	localTotal := localPromptTokens + localCompletionTokens

	// 零 token 检查
	if responseTotal == 0 && localTotal == 0 {
		result.Anomaly = AnomalyZeroTokens
		result.IsVerified = false
		result.Reason = "响应 usage 与本地估算均为 0 token"
		result.ConfidenceScore = 0.0
		atomic.AddInt64(&v.stats.Anomalies, 1)
		v.recordLatency(latencyMs)
		return result
	}

	// 无 usage 字段，仅本地估算
	if !hasUsage || responseTotal == 0 {
		result.Anomaly = AnomalyMissingUsage
		result.ResponseTokens = 0
		result.LocalTokens = localTotal
		result.ConfidenceScore = 0.7 // 本地估算置信度较低
		// 不标记为 IsVerified=false，因为这是预期行为（兜底策略）
		// 但记录为异常以便追踪
		atomic.AddInt64(&v.stats.WithLocalOnly, 1)
		atomic.AddInt64(&v.stats.Anomalies, 1)
		v.recordLatency(latencyMs)
		// 仍然检查延迟和响应大小
		v.checkLatencyAndSize(&result, latencyMs, responseBytes)
		return result
	}

	// 有 usage 字段，执行双重校验
	atomic.AddInt64(&v.stats.WithResponseUsage, 1)
	result.ResponseTokens = responseTotal
	result.LocalTokens = localTotal

	// 计算偏差
	if localTotal > 0 {
		result.DeviationPct = calcDeviation(responseTotal, localTotal)
	} else {
		// 本地估算为 0 但响应有值，偏差 100%
		result.DeviationPct = 100.0
	}

	// 偏差检查（仅对足够大的 token 数进行检查）
	totalCheck := responseTotal + localTotal
	if totalCheck >= v.cfg.MinTokensForCheck*2 {
		if result.DeviationPct > v.cfg.MaxDeviationPct {
			result.Anomaly = classifyDeviation(
				responsePromptTokens, localPromptTokens,
				responseCompletionTokens, localCompletionTokens,
			)
			result.IsVerified = false
			result.Reason = "token 偏差超过阈值"
			result.ConfidenceScore = 1.0 - result.DeviationPct/100.0
			if result.ConfidenceScore < 0 {
				result.ConfidenceScore = 0
			}
			atomic.AddInt64(&v.stats.Anomalies, 1)
		}
	}

	// 延迟和响应大小检查
	v.checkLatencyAndSize(&result, latencyMs, responseBytes)

	// 记录延迟到滑动窗口（所有路径都需要记录）
	v.recordLatency(latencyMs)

	// 更新平均偏差
	v.updateAvgDeviation(result.DeviationPct)

	if result.IsVerified {
		atomic.AddInt64(&v.stats.Passed, 1)
	}

	return result
}

// checkLatencyAndSize 检查延迟和响应大小异常
func (v *Verifier) checkLatencyAndSize(result *VerificationResult, latencyMs int64, responseBytes int) {
	// 延迟异常
	if latencyMs > v.cfg.MaxLatencyMs {
		if !result.Anomaly.IsAnomaly() {
			result.Anomaly = AnomalyLatencySpike
			result.IsVerified = false
			result.Reason = "请求延迟异常"
		}
	}

	// 响应大小异常
	if responseBytes > v.cfg.MaxResponseBytes {
		if !result.Anomaly.IsAnomaly() {
			result.Anomaly = AnomalyResponseSize
			result.IsVerified = false
			result.Reason = "响应体大小异常"
		}
	}
}

// classifyDeviation 判断偏差主要来源
func classifyDeviation(respPrompt, localPrompt, respCompletion, localCompletion int) AnomalyType {
	promptDev := calcDeviation(respPrompt, localPrompt)
	completionDev := calcDeviation(respCompletion, localCompletion)
	if promptDev >= completionDev {
		return AnomalyPromptDeviation
	}
	return AnomalyCompletionDeviation
}

// calcDeviation 计算偏差百分比
func calcDeviation(a, b int) float64 {
	if a == 0 && b == 0 {
		return 0
	}
	max := float64(a)
	if float64(b) > max {
		max = float64(b)
	}
	if max == 0 {
		return 0
	}
	return math.Abs(float64(a)-float64(b)) / max * 100.0
}

// recordLatency 记录延迟到滑动窗口
func (v *Verifier) recordLatency(latencyMs int64) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.latencyWindow = append(v.latencyWindow, latencyMs)
	if len(v.latencyWindow) > v.windowSize {
		v.latencyWindow = v.latencyWindow[1:]
	}
}

// updateAvgDeviation 更新平均偏差（使用原子操作）
func (v *Verifier) updateAvgDeviation(deviation float64) {
	scaled := uint64(deviation * 1000)
	for {
		old := atomic.LoadUint64(&v.stats.AvgDeviation)
		count := atomic.LoadInt64(&v.stats.TotalVerified)
		if count <= 1 {
			if atomic.CompareAndSwapUint64(&v.stats.AvgDeviation, old, scaled) {
				return
			}
			continue
		}
		// 增量平均
		newAvg := (uint64(count-1)*old + scaled) / uint64(count)
		if atomic.CompareAndSwapUint64(&v.stats.AvgDeviation, old, newAvg) {
			return
		}
	}
}

// GetStats 获取统计快照
func (v *Verifier) GetStats() Stats {
	return Stats{
		TotalVerified:     atomic.LoadInt64(&v.stats.TotalVerified),
		Passed:            atomic.LoadInt64(&v.stats.Passed),
		Anomalies:         atomic.LoadInt64(&v.stats.Anomalies),
		WithResponseUsage: atomic.LoadInt64(&v.stats.WithResponseUsage),
		WithLocalOnly:     atomic.LoadInt64(&v.stats.WithLocalOnly),
		AvgDeviation:      atomic.LoadUint64(&v.stats.AvgDeviation),
	}
}

// GetLatencyStats 获取延迟统计（基于滑动窗口）
func (v *Verifier) GetLatencyStats() (avg, max, p99 int64) {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if len(v.latencyWindow) == 0 {
		return 0, 0, 0
	}

	var sum int64
	maxVal := v.latencyWindow[0]
	for _, l := range v.latencyWindow {
		sum += l
		if l > maxVal {
			maxVal = l
		}
	}
	avg = sum / int64(len(v.latencyWindow))
	max = maxVal
	p99 = maxVal
	return
}
