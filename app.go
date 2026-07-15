package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"ticktoken/internal/adapters"
	"ticktoken/internal/apikey"
	"ticktoken/internal/cache"
	"ticktoken/internal/config"
	"ticktoken/internal/counter"
	"ticktoken/internal/proxy"
	"ticktoken/internal/storage"
	"ticktoken/internal/verifier"
)

// App 桌面应用主结构
type App struct {
	ctx       context.Context
	cfg       *config.Config
	certMgr   *proxy.CertManager
	store     *storage.Store
	counter   *counter.Counter
	fpDB      *adapters.FingerprintDB
	mitmProxy *proxy.MITMProxy
	// v2 优化新增
	apiKeyMonitor *apikey.Monitor       // API Key 调用监控
	verifier      *verifier.Verifier    // 双重校验器
	fileWatcher   *adapters.FileWatcher // CLI 工具实时监控
	// v3 优化新增
	stopCleanup func() // 数据清理停止函数
}

// NewApp 创建应用实例
func NewApp() *App {
	return &App{}
}

// Startup Wails 启动回调，初始化所有后端服务
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx

	// 加载配置（失败时降级到默认配置，不退出应用，保证窗口可见）
	cfg, err := config.Load()
	if err != nil {
		log.Printf("[App] 加载配置失败（降级到默认配置）: %v", err)
		cfg = config.Default()
	}
	a.cfg = cfg

	// 初始化 CA 证书管理器（失败时降级，代理功能不可用但窗口可见）
	certMgr, err := proxy.NewCertManager(cfg.CADir)
	if err != nil {
		log.Printf("[App] CA 证书管理器初始化失败（降级模式）: %v", err)
	} else {
		a.certMgr = certMgr
	}

	// 初始化存储（失败时降级，事件不持久化但窗口可见）
	store, err := storage.NewStore(cfg.DBPath)
	if err != nil {
		log.Printf("[App] 存储初始化失败（降级模式）: %v", err)
	} else {
		a.store = store
	}

	// 初始化计数引擎和指纹库
	a.counter = counter.NewCounter()
	a.fpDB = adapters.NewFingerprintDB()

	// v2: 初始化用户设置（语言等）
	if err := config.InitSettings(cfg.DataDir); err != nil {
		log.Printf("[App] 设置初始化失败（非致命，使用默认值）: %v", err)
	}

	// v2: 初始化 API Key 监控器
	a.apiKeyMonitor = apikey.NewMonitor()

	// v2: 初始化双重校验器（偏差阈值 15%）
	a.verifier = verifier.NewVerifier(verifier.DefaultConfig())

	// v2: 初始化 CLI 工具实时文件监控（1秒间隔）
	a.fileWatcher, err = adapters.NewFileWatcher(1 * time.Second)
	if err != nil {
		log.Printf("[App] 文件监控器初始化失败（非致命）: %v", err)
	} else {
		// 启动 CLI 工具监控
		if err := a.fileWatcher.Start(); err != nil {
			log.Printf("[App] 文件监控启动失败: %v", err)
		} else {
			// 消费 CLI 监控事件
			go a.consumeWatchEvents()
		}
	}

	// 创建 MITM 代理，payload 处理回调（v2: 异步管道 + API Key 监控 + 双重校验）
	proxyHandler := func(host string, reqBody []byte, respBody []byte, req *http.Request, resp *http.Response) {
		// 异步处理，避免阻塞代理转发（延迟控制在 1 秒内）
		go a.processCapturedRequest(host, reqBody, respBody, req, resp)
	}

	a.mitmProxy = proxy.NewMITMProxy(cfg.ProxyAddr, a.certMgr, proxyHandler)
	if a.certMgr != nil {
		if err := a.mitmProxy.Start(); err != nil {
			log.Printf("[App] 代理启动失败（降级模式，监控不可用）: %v", err)
		}
	} else {
		log.Printf("[App] CA 证书不可用，跳过代理启动（降级模式）")
	}

	// v3: 启动数据自动清理（保留 90 天，每天清理一次）
	if a.store != nil {
		a.stopCleanup = a.store.StartAutoCleanup(90)
	}

	log.Printf("[App] TickToken 桌面版已启动，模式: %s（v3 优化版 - H2/SSE/一键配置）", cfg.Mode())
}

// processCapturedRequest 处理捕获的请求（异步管道）
// 集成 API Key 监控、双重校验、异常检测
func (a *App) processCapturedRequest(host string, reqBody []byte, respBody []byte, req *http.Request, resp *http.Response) {
	// 降级模式保护：store 或其他组件未初始化时跳过
	if a.store == nil || a.counter == nil || a.verifier == nil {
		return
	}

	startTime := time.Now()
	tool := a.fpDB.IdentifyFromRequest(req)
	model := counter.ExtractModelFromRequest(reqBody)

	// v2: API Key 监控 - 启动请求生命周期跟踪
	requestID := a.apiKeyMonitor.StartRequest(host, req, tool)

	// v2: 双重校验计数（同时获取 response usage 和本地 tokenizer 估算）
	finalCount, localCount := a.counter.CountWithVerification(reqBody, respBody, model)
	cacheInfo := cache.ParseFromResponse(respBody, finalCount.PromptTokens)

	// v2: 执行双重校验
	statusCode := 0
	if resp != nil {
		statusCode = resp.StatusCode
	}
	latencyMs := time.Since(startTime).Milliseconds()

	verification := a.verifier.Verify(
		finalCount.PromptTokens, finalCount.CompletionTokens,
		localCount.PromptTokens, localCount.CompletionTokens,
		finalCount.HasUsage,
		latencyMs,
		len(respBody),
	)

	// 构建事件记录（包含异常标记）
	event := &storage.TokenEvent{
		Timestamp:        time.Now(),
		Tool:             tool,
		Model:            model,
		PromptTokens:     finalCount.PromptTokens,
		CompletionTokens: finalCount.CompletionTokens,
		CacheHit:         cacheInfo.CacheHit,
		CacheMiss:        cacheInfo.CacheMiss,
		CacheCreation:    cacheInfo.CacheCreation,
		Source:           finalCount.Source,
		Tokenizer:        finalCount.Tokenizer,
		IsAnomaly:        verification.Anomaly.IsAnomaly(),
		AnomalyType:      verification.Anomaly.String(),
		DeviationPct:     verification.DeviationPct,
		LatencyMs:        latencyMs,
		Provider:         a.detectProvider(req, host),
	}

	// 写入存储
	if err := a.store.InsertEvent(event); err != nil {
		log.Printf("[Proxy] 写入事件失败: %v", err)
	}

	// v2: 完成 API Key 监控生命周期
	if requestID != "" {
		a.apiKeyMonitor.CompleteRequest(requestID, statusCode, model,
			finalCount.PromptTokens, finalCount.CompletionTokens,
			cacheInfo.CacheHit, cacheInfo.CacheMiss, finalCount.Source)

		// 标记异常
		if verification.Anomaly.IsAnomaly() {
			a.apiKeyMonitor.MarkAnomaly(requestID, verification.Reason)
		}
	}

	// 通过 Wails 运行时推送事件到前端
	wailsRuntime.EventsEmit(a.ctx, "token:event", event)

	// 异常事件额外推送
	if verification.Anomaly.IsAnomaly() {
		wailsRuntime.EventsEmit(a.ctx, "anomaly:event", map[string]interface{}{
			"timestamp":     event.Timestamp,
			"tool":          tool,
			"model":         model,
			"anomalyType":   verification.Anomaly.String(),
			"deviationPct":  verification.DeviationPct,
			"reason":        verification.Reason,
			"responseTokens": verification.ResponseTokens,
			"localTokens":    verification.LocalTokens,
		})
		log.Printf("[Proxy] 异常检测: tool=%s model=%s type=%s deviation=%.2f%%",
			tool, model, verification.Anomaly.String(), verification.DeviationPct)
	}
}

// consumeWatchEvents 消费 CLI 工具监控事件
func (a *App) consumeWatchEvents() {
	for event := range a.fileWatcher.Events() {
		// 将 CLI 日志事件写入存储
		tokenEvent := &storage.TokenEvent{
			Timestamp:        event.Timestamp,
			Tool:             event.Source,
			Model:            event.Model,
			PromptTokens:     event.PromptTokens,
			CompletionTokens: event.CompletionTokens,
			Source:           "log_watcher",
			Tokenizer:        "response",
		}

		if err := a.store.InsertEvent(tokenEvent); err != nil {
			log.Printf("[Watcher] 写入事件失败: %v", err)
			continue
		}

		// 推送到前端
		wailsRuntime.EventsEmit(a.ctx, "token:event", tokenEvent)
	}
}

// detectProvider 从请求中检测 API provider
func (a *App) detectProvider(req *http.Request, host string) string {
	detection := apikey.DetectAPIKey(req)
	if detection.Present {
		return detection.Provider
	}
	// 无 API Key 时根据 host 推断
	return apikey.DetectProviderByHost(host)
}

// Shutdown Wails 关闭回调
func (a *App) Shutdown(ctx context.Context) {
	if a.stopCleanup != nil {
		a.stopCleanup()
	}
	if a.fileWatcher != nil {
		a.fileWatcher.Stop()
	}
	if a.store != nil {
		a.store.Close()
	}
}

// StatusInfo 状态信息（暴露给前端）
type StatusInfo struct {
	Mode              string  `json:"mode"`
	ProxyAddr         string  `json:"proxyAddr"`
	DataDir           string  `json:"dataDir"`
	CACertPath        string  `json:"caCertPath"`
	Verbose           bool    `json:"verbose"`
	ToolCount         int     `json:"toolCount"`
	// v2 新增
	InflightRequests  int64   `json:"inflightRequests"`
	TotalAPIKeyReqs   int64   `json:"totalAPIKeyReqs"`
	VerificationRate  float64 `json:"verificationRate"`
	AvgDeviationPct   float64 `json:"avgDeviationPct"`
	WatchedFilesCount int     `json:"watchedFilesCount"`
}

// GetStatus 获取应用状态
func (a *App) GetStatus() StatusInfo {
	info := StatusInfo{
		Mode:      "degraded",
		ProxyAddr: "-",
		DataDir:   "-",
	}
	if a.cfg != nil {
		info.Mode = a.cfg.Mode()
		info.ProxyAddr = a.cfg.ProxyAddr
		info.DataDir = a.cfg.DataDir
		info.Verbose = a.cfg.Verbose
	}
	if a.certMgr != nil {
		info.CACertPath = a.certMgr.GetCACertPath()
	}
	if a.fpDB != nil {
		info.ToolCount = len(a.fpDB.ListTools())
	}
	if a.apiKeyMonitor != nil {
		monitorStats := a.apiKeyMonitor.GetStats()
		info.InflightRequests = int64(a.apiKeyMonitor.InflightCount())
		info.TotalAPIKeyReqs = monitorStats.TotalRequests
	}
	if a.verifier != nil {
		verifierStats := a.verifier.GetStats()
		info.VerificationRate = verifierStats.AccuracyRate()
		info.AvgDeviationPct = verifierStats.AverageDeviation()
	}
	if a.fileWatcher != nil {
		info.WatchedFilesCount = len(a.fileWatcher.GetWatchedFiles())
	}
	return info
}

// GetTimeSeries 获取时间序列数据
func (a *App) GetTimeSeries(hours int) []storage.TimeSeriesPoint {
	if a.store == nil {
		return []storage.TimeSeriesPoint{}
	}
	if hours <= 0 || hours > 720 {
		hours = 24
	}
	end := time.Now()
	start := end.Add(-time.Duration(hours) * time.Hour)
	points, err := a.store.QueryTimeSeries(start, end)
	if err != nil {
		log.Printf("[App] 查询时间序列失败: %v", err)
		return []storage.TimeSeriesPoint{}
	}
	return points
}

// GetAggregate 获取聚合数据
func (a *App) GetAggregate(dimension string, hours int) []storage.AggregationItem {
	if a.store == nil {
		return []storage.AggregationItem{}
	}
	if dimension != "model" && dimension != "tool" {
		dimension = "model"
	}
	if hours <= 0 || hours > 720 {
		hours = 24
	}
	end := time.Now()
	start := end.Add(-time.Duration(hours) * time.Hour)
	items, err := a.store.QueryByDimension(dimension, start, end)
	if err != nil {
		log.Printf("[App] 查询聚合失败: %v", err)
		return []storage.AggregationItem{}
	}
	return items
}

// GetRecentEvents 获取最近事件
func (a *App) GetRecentEvents(limit int) []storage.TokenEvent {
	if a.store == nil {
		return []storage.TokenEvent{}
	}
	if limit <= 0 || limit > 1000 {
		limit = 50
	}
	events, err := a.store.Query(storage.QueryFilter{})
	if err != nil {
		return []storage.TokenEvent{}
	}
	if len(events) > limit {
		events = events[:limit]
	}
	return events
}

// SummaryStats 汇总统计
type SummaryStats struct {
	TotalPrompt     int `json:"totalPrompt"`
	TotalCompletion int `json:"totalCompletion"`
	TotalCacheHit   int `json:"totalCacheHit"`
	TotalEvents     int `json:"totalEvents"`
}

// GetSummary 获取指定时间窗口内的汇总统计
func (a *App) GetSummary(hours int) SummaryStats {
	if a.store == nil {
		return SummaryStats{}
	}
	if hours <= 0 || hours > 720 {
		hours = 24
	}
	end := time.Now()
	start := end.Add(-time.Duration(hours) * time.Hour)

	stats := SummaryStats{}

	points, err := a.store.QueryTimeSeries(start, end)
	if err == nil {
		for _, p := range points {
			stats.TotalPrompt += p.PromptTokens
			stats.TotalCompletion += p.CompletionTokens
			stats.TotalEvents++
		}
	}

	items, err := a.store.QueryByDimension("model", start, end)
	if err == nil {
		for _, item := range items {
			stats.TotalCacheHit += item.CacheHit
		}
	}

	return stats
}

// v2 新增 API：趋势分析、异常查询、监控统计

// GetTrend 获取趋势分析数据
// granularity: "hour" 或 "day"
func (a *App) GetTrend(hours int, granularity string) []storage.TrendPoint {
	if a.store == nil {
		return []storage.TrendPoint{}
	}
	if hours <= 0 || hours > 720 {
		hours = 24
	}
	if granularity != "hour" && granularity != "day" {
		granularity = "hour"
	}
	end := time.Now()
	start := end.Add(-time.Duration(hours) * time.Hour)
	points, err := a.store.QueryTrend(start, end, granularity)
	if err != nil {
		log.Printf("[App] 查询趋势失败: %v", err)
		return []storage.TrendPoint{}
	}
	return points
}

// GetAnomalyStats 获取异常统计
func (a *App) GetAnomalyStats(hours int) *storage.AnomalyStats {
	if a.store == nil {
		return &storage.AnomalyStats{ByType: make(map[string]int64)}
	}
	if hours <= 0 || hours > 720 {
		hours = 24
	}
	end := time.Now()
	start := end.Add(-time.Duration(hours) * time.Hour)
	stats, err := a.store.QueryAnomalyStats(start, end)
	if err != nil {
		log.Printf("[App] 查询异常统计失败: %v", err)
		return &storage.AnomalyStats{ByType: make(map[string]int64)}
	}
	return stats
}

// GetAnomalies 获取异常事件列表
func (a *App) GetAnomalies(limit int) []storage.TokenEvent {
	if a.store == nil {
		return []storage.TokenEvent{}
	}
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	events, err := a.store.QueryAnomalies(limit)
	if err != nil {
		log.Printf("[App] 查询异常事件失败: %v", err)
		return []storage.TokenEvent{}
	}
	return events
}

// VerificationStats 校验统计（暴露给前端）
type VerificationStats struct {
	TotalVerified     int64   `json:"totalVerified"`
	Passed            int64   `json:"passed"`
	Anomalies         int64   `json:"anomalies"`
	AccuracyRate      float64 `json:"accuracyRate"`
	AvgDeviationPct   float64 `json:"avgDeviationPct"`
	WithResponseUsage int64   `json:"withResponseUsage"`
	WithLocalOnly     int64   `json:"withLocalOnly"`
	AvgLatencyMs      int64   `json:"avgLatencyMs"`
	MaxLatencyMs      int64   `json:"maxLatencyMs"`
}

// GetVerificationStats 获取校验统计
func (a *App) GetVerificationStats() VerificationStats {
	if a.verifier == nil {
		return VerificationStats{}
	}
	stats := a.verifier.GetStats()
	avgLat, maxLat, _ := a.verifier.GetLatencyStats()
	return VerificationStats{
		TotalVerified:     stats.TotalVerified,
		Passed:            stats.Passed,
		Anomalies:         stats.Anomalies,
		AccuracyRate:      stats.AccuracyRate(),
		AvgDeviationPct:   stats.AverageDeviation(),
		WithResponseUsage: stats.WithResponseUsage,
		WithLocalOnly:     stats.WithLocalOnly,
		AvgLatencyMs:      avgLat,
		MaxLatencyMs:      maxLat,
	}
}

// APIKeyMonitorStats API Key 监控统计（暴露给前端）
type APIKeyMonitorStats struct {
	TotalRequests     int64 `json:"totalRequests"`
	CompletedRequests int64 `json:"completedRequests"`
	FailedRequests    int64 `json:"failedRequests"`
	AnomalyRequests   int64 `json:"anomalyRequests"`
	InflightCount     int   `json:"inflightCount"`
}

// GetAPIKeyMonitorStats 获取 API Key 监控统计
func (a *App) GetAPIKeyMonitorStats() APIKeyMonitorStats {
	if a.apiKeyMonitor == nil {
		return APIKeyMonitorStats{}
	}
	stats := a.apiKeyMonitor.GetStats()
	return APIKeyMonitorStats{
		TotalRequests:     stats.TotalRequests,
		CompletedRequests: stats.CompletedRequests,
		FailedRequests:    stats.FailedRequests,
		AnomalyRequests:   stats.AnomalyRequests,
		InflightCount:     a.apiKeyMonitor.InflightCount(),
	}
}

// OpenInFileManager 在系统文件管理器中打开路径
func (a *App) OpenInFileManager(path string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("explorer", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	if err := cmd.Start(); err != nil {
		log.Printf("[App] 打开文件管理器失败: %v", err)
	}
}

// OpenCACert 在文件管理器中定位 CA 证书
func (a *App) OpenCACert() {
	if a.certMgr == nil {
		return
	}
	a.OpenInFileManager(a.certMgr.GetCACertPath())
}

// OpenDataDir 在文件管理器中打开数据目录
func (a *App) OpenDataDir() {
	if a.cfg == nil {
		return
	}
	a.OpenInFileManager(a.cfg.DataDir)
}

// LanguageInfo 语言信息（暴露给前端）
type LanguageInfo struct {
	Current    string   `json:"current"`    // 当前语言代码 "zh" / "en"
	Available  []string `json:"available"`  // 支持的语言列表
}

// GetLanguage 获取当前语言设置
func (a *App) GetLanguage() LanguageInfo {
	s := config.GetSettings()
	return LanguageInfo{
		Current:   s.Language,
		Available: config.SupportedLanguages,
	}
}

// SetLanguage 设置语言并持久化
// 返回更新后的语言信息
func (a *App) SetLanguage(lang string) (LanguageInfo, error) {
	if !config.IsValidLanguage(lang) {
		return LanguageInfo{}, fmt.Errorf("不支持的语言代码: %s", lang)
	}
	if err := config.UpdateSettings(config.Settings{Language: lang}); err != nil {
		return LanguageInfo{}, fmt.Errorf("保存语言设置失败: %w", err)
	}
	log.Printf("[App] 语言已切换为: %s", lang)
	return a.GetLanguage(), nil
}
