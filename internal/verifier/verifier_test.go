package verifier

import (
	"sync"
	"testing"
)

// TestVerify_Match 测试 token 一致时通过校验
func TestVerify_Match(t *testing.T) {
	v := NewVerifier(DefaultConfig())
	result := v.Verify(100, 50, 100, 50, true, 500, 1000)

	if !result.IsVerified {
		t.Error("一致时应通过校验")
	}
	if result.Anomaly != AnomalyNone {
		t.Errorf("Anomaly 期望 none，得到 %s", result.Anomaly)
	}
	if result.DeviationPct != 0 {
		t.Errorf("DeviationPct 期望 0，得到 %.2f", result.DeviationPct)
	}
	if result.ConfidenceScore != 1.0 {
		t.Errorf("ConfidenceScore 期望 1.0，得到 %.2f", result.ConfidenceScore)
	}
}

// TestVerify_PromptDeviation 测试 prompt token 偏差检测
func TestVerify_PromptDeviation(t *testing.T) {
	v := NewVerifier(DefaultConfig())
	// response: prompt=100, completion=50 → total=150
	// local: prompt=200, completion=50 → total=250
	// deviation = |150-250|/250 = 40% > 15%
	result := v.Verify(100, 50, 200, 50, true, 500, 1000)

	if result.IsVerified {
		t.Error("偏差超过阈值不应通过校验")
	}
	if result.Anomaly != AnomalyPromptDeviation {
		t.Errorf("Anomaly 期望 prompt_deviation，得到 %s", result.Anomaly)
	}
	if result.DeviationPct <= 15.0 {
		t.Errorf("DeviationPct 应 > 15%%，得到 %.2f%%", result.DeviationPct)
	}
}

// TestVerify_CompletionDeviation 测试 completion token 偏差检测
func TestVerify_CompletionDeviation(t *testing.T) {
	v := NewVerifier(DefaultConfig())
	// response: prompt=100, completion=200 → total=300
	// local: prompt=100, completion=50 → total=150
	// deviation = |300-150|/300 = 50%
	result := v.Verify(100, 200, 100, 50, true, 500, 1000)

	if result.IsVerified {
		t.Error("偏差超过阈值不应通过校验")
	}
	if result.Anomaly != AnomalyCompletionDeviation {
		t.Errorf("Anomaly 期望 completion_deviation，得到 %s", result.Anomaly)
	}
}

// TestVerify_MissingUsage 测试缺少 usage 字段
func TestVerify_MissingUsage(t *testing.T) {
	v := NewVerifier(DefaultConfig())
	result := v.Verify(0, 0, 100, 50, false, 500, 1000)

	if result.Anomaly != AnomalyMissingUsage {
		t.Errorf("Anomaly 期望 missing_usage，得到 %s", result.Anomaly)
	}
	if result.ResponseTokens != 0 {
		t.Errorf("ResponseTokens 期望 0，得到 %d", result.ResponseTokens)
	}
	if result.LocalTokens != 150 {
		t.Errorf("LocalTokens 期望 150，得到 %d", result.LocalTokens)
	}
}

// TestVerify_ZeroTokens 测试两个来源都返回 0
func TestVerify_ZeroTokens(t *testing.T) {
	v := NewVerifier(DefaultConfig())
	result := v.Verify(0, 0, 0, 0, false, 500, 1000)

	if result.Anomaly != AnomalyZeroTokens {
		t.Errorf("Anomaly 期望 zero_tokens，得到 %s", result.Anomaly)
	}
	if result.IsVerified {
		t.Error("零 token 不应通过校验")
	}
}

// TestVerify_NegativeTokens 测试负数 token
func TestVerify_NegativeTokens(t *testing.T) {
	v := NewVerifier(DefaultConfig())
	result := v.Verify(-1, 50, 100, 50, true, 500, 1000)

	if result.Anomaly != AnomalyNegativeTokens {
		t.Errorf("Anomaly 期望 negative_tokens，得到 %s", result.Anomaly)
	}
}

// TestVerify_LatencySpike 测试延迟异常
func TestVerify_LatencySpike(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxLatencyMs = 1000 // 1秒阈值
	v := NewVerifier(cfg)

	result := v.Verify(100, 50, 100, 50, true, 5000, 1000) // 5秒延迟
	if result.Anomaly != AnomalyLatencySpike {
		t.Errorf("Anomaly 期望 latency_spike，得到 %s", result.Anomaly)
	}
}

// TestVerify_ResponseSizeAnomaly 测试响应大小异常
func TestVerify_ResponseSizeAnomaly(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxResponseBytes = 1024 // 1KB 阈值
	v := NewVerifier(cfg)

	result := v.Verify(100, 50, 100, 50, true, 500, 2048) // 2KB 响应
	if result.Anomaly != AnomalyResponseSize {
		t.Errorf("Anomaly 期望 response_size_anomaly，得到 %s", result.Anomaly)
	}
}

// TestVerify_SmallTokenSkipped 测试小 token 数跳过偏差检查
func TestVerify_SmallTokenSkipped(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MinTokensForCheck = 100 // 阈值 100
	v := NewVerifier(cfg)

	// response total = 3+2 = 5, local total = 3+2 = 5
	// totalCheck = 5+5 = 10 < 200 (MinTokensForCheck*2)，跳过偏差检查
	result := v.Verify(3, 2, 3, 2, true, 500, 1000)
	if !result.IsVerified {
		t.Error("小 token 数应跳过偏差检查并通过校验")
	}
}

// TestVerify_AccuracyRate 测试准确率统计
func TestVerify_AccuracyRate(t *testing.T) {
	v := NewVerifier(DefaultConfig())

	// 10 个一致的请求
	for i := 0; i < 10; i++ {
		v.Verify(100, 50, 100, 50, true, 500, 1000)
	}
	// 1 个偏差请求
	v.Verify(100, 50, 200, 50, true, 500, 1000)

	stats := v.GetStats()
	if stats.TotalVerified != 11 {
		t.Errorf("TotalVerified 期望 11，得到 %d", stats.TotalVerified)
	}
	if stats.Passed != 10 {
		t.Errorf("Passed 期望 10，得到 %d", stats.Passed)
	}
	if stats.Anomalies != 1 {
		t.Errorf("Anomalies 期望 1，得到 %d", stats.Anomalies)
	}
	// accuracyRate 应为 10/11 ≈ 90.9%
	rate := stats.AccuracyRate()
	if rate < 90.0 || rate > 91.0 {
		t.Errorf("AccuracyRate 期望 ~90.9%%，得到 %.2f%%", rate)
	}
}

// TestVerify_Concurrent 测试并发校验安全性
func TestVerify_Concurrent(t *testing.T) {
	v := NewVerifier(DefaultConfig())
	var wg sync.WaitGroup

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v.Verify(100, 50, 100, 50, true, 500, 1000)
		}()
	}
	wg.Wait()

	stats := v.GetStats()
	if stats.TotalVerified != 1000 {
		t.Errorf("TotalVerified 期望 1000，得到 %d", stats.TotalVerified)
	}
	if stats.Passed != 1000 {
		t.Errorf("Passed 期望 1000，得到 %d", stats.Passed)
	}
}

// TestVerify_WithResponseUsageCount 测试 WithResponseUsage 统计
func TestVerify_WithResponseUsageCount(t *testing.T) {
	v := NewVerifier(DefaultConfig())

	// 有 usage
	v.Verify(100, 50, 100, 50, true, 500, 1000)
	v.Verify(100, 50, 100, 50, true, 500, 1000)
	// 无 usage
	v.Verify(0, 0, 100, 50, false, 500, 1000)

	stats := v.GetStats()
	if stats.WithResponseUsage != 2 {
		t.Errorf("WithResponseUsage 期望 2，得到 %d", stats.WithResponseUsage)
	}
	if stats.WithLocalOnly != 1 {
		t.Errorf("WithLocalOnly 期望 1，得到 %d", stats.WithLocalOnly)
	}
}

// TestAnomalyType_IsAnomaly 测试 IsAnomaly 方法
func TestAnomalyType_IsAnomaly(t *testing.T) {
	if AnomalyNone.IsAnomaly() {
		t.Error("AnomalyNone 不应是异常")
	}
	if !AnomalyPromptDeviation.IsAnomaly() {
		t.Error("AnomalyPromptDeviation 应是异常")
	}
	if !AnomalyZeroTokens.IsAnomaly() {
		t.Error("AnomalyZeroTokens 应是异常")
	}
}

// TestGetLatencyStats 测试延迟统计
func TestGetLatencyStats(t *testing.T) {
	v := NewVerifier(DefaultConfig())

	// 直接调用 Verify 记录延迟
	v.Verify(100, 50, 100, 50, true, 100, 1000)
	v.Verify(100, 50, 100, 50, true, 200, 1000)
	v.Verify(100, 50, 100, 50, true, 300, 1000)

	avg, max, p99 := v.GetLatencyStats()
	// 延迟窗口应包含记录的值
	if max < 100 {
		t.Errorf("Max latency 应 >= 100，得到 %d (avg=%d, p99=%d)", max, avg, p99)
	}
}
