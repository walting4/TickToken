package main

import (
	"context"
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"ticktoken/internal/adapters"
	"ticktoken/internal/cache"
	"ticktoken/internal/config"
	"ticktoken/internal/counter"
	"ticktoken/internal/proxy"
	"ticktoken/internal/storage"
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
}

// NewApp 创建应用实例
func NewApp() *App {
	return &App{}
}

// Startup Wails 启动回调，初始化所有后端服务
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx

	// 加载配置
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[App] 加载配置失败: %v", err)
	}
	a.cfg = cfg

	// 初始化 CA 证书管理器
	certMgr, err := proxy.NewCertManager(cfg.CADir)
	if err != nil {
		log.Fatalf("[App] CA 证书管理器初始化失败: %v", err)
	}
	a.certMgr = certMgr

	// 初始化存储
	store, err := storage.NewStore(cfg.DBPath)
	if err != nil {
		log.Fatalf("[App] 存储初始化失败: %v", err)
	}
	a.store = store

	// 初始化计数引擎和指纹库
	a.counter = counter.NewCounter()
	a.fpDB = adapters.NewFingerprintDB()

	// 创建 MITM 代理，payload 处理回调
	proxyHandler := func(host string, reqBody []byte, respBody []byte, req *http.Request, resp *http.Response) {
		tool := a.fpDB.IdentifyFromRequest(req)
		model := counter.ExtractModelFromRequest(reqBody)
		tc := a.counter.Count(reqBody, respBody, model)
		cacheInfo := cache.ParseFromResponse(respBody, tc.PromptTokens)

		event := &storage.TokenEvent{
			Timestamp:        time.Now(),
			Tool:             tool,
			Model:            model,
			PromptTokens:     tc.PromptTokens,
			CompletionTokens: tc.CompletionTokens,
			CacheHit:         cacheInfo.CacheHit,
			CacheMiss:        cacheInfo.CacheMiss,
			CacheCreation:    cacheInfo.CacheCreation,
			Source:           tc.Source,
			Tokenizer:        tc.Tokenizer,
		}

		if err := a.store.InsertEvent(event); err != nil {
			log.Printf("[Proxy] 写入事件失败: %v", err)
		}

		// 通过 Wails 运行时推送事件到前端
		wailsRuntime.EventsEmit(ctx, "token:event", event)
	}

	a.mitmProxy = proxy.NewMITMProxy(cfg.ProxyAddr, a.certMgr, proxyHandler)
	if err := a.mitmProxy.Start(); err != nil {
		log.Fatalf("[App] 代理启动失败: %v", err)
	}

	log.Printf("[App] TickToken 桌面版已启动，模式: %s", cfg.Mode())
}

// Shutdown Wails 关闭回调
func (a *App) Shutdown(ctx context.Context) {
	if a.store != nil {
		a.store.Close()
	}
}

// StatusInfo 状态信息（暴露给前端）
type StatusInfo struct {
	Mode       string `json:"mode"`
	ProxyAddr  string `json:"proxyAddr"`
	DataDir    string `json:"dataDir"`
	CACertPath string `json:"caCertPath"`
	Verbose    bool   `json:"verbose"`
	ToolCount  int    `json:"toolCount"`
}

// GetStatus 获取应用状态
func (a *App) GetStatus() StatusInfo {
	return StatusInfo{
		Mode:       a.cfg.Mode(),
		ProxyAddr:  a.cfg.ProxyAddr,
		DataDir:    a.cfg.DataDir,
		CACertPath: a.certMgr.GetCACertPath(),
		Verbose:    a.cfg.Verbose,
		ToolCount:  len(a.fpDB.ListTools()),
	}
}

// GetTimeSeries 获取时间序列数据
func (a *App) GetTimeSeries(hours int) []storage.TimeSeriesPoint {
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
	if hours <= 0 || hours > 720 {
		hours = 24
	}
	end := time.Now()
	start := end.Add(-time.Duration(hours) * time.Hour)

	stats := SummaryStats{}

	// 从时间序列获取 prompt/completion 总量
	points, err := a.store.QueryTimeSeries(start, end)
	if err == nil {
		for _, p := range points {
			stats.TotalPrompt += p.PromptTokens
			stats.TotalCompletion += p.CompletionTokens
			stats.TotalEvents++
		}
	}

	// 从聚合数据获取 cache hit 总量
	items, err := a.store.QueryByDimension("model", start, end)
	if err == nil {
		for _, item := range items {
			stats.TotalCacheHit += item.CacheHit
		}
	}

	return stats
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
	a.OpenInFileManager(a.certMgr.GetCACertPath())
}

// OpenDataDir 在文件管理器中打开数据目录
func (a *App) OpenDataDir() {
	a.OpenInFileManager(a.cfg.DataDir)
}
