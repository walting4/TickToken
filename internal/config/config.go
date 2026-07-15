package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// Config 应用配置
type Config struct {
	ProxyAddr     string // 代理监听地址，默认 127.0.0.1:8899
	DashboardAddr string // 仪表盘监听地址，默认 127.0.0.1:8900
	DataDir       string // 数据目录，默认 ~/.ticktoken
	DBPath        string // SQLite 数据库路径
	CADir         string // CA 证书目录
	APIKey        string // 可选的 API key（为空则纯被动观测模式）
	Verbose       bool   // 详细日志
}

// Mode 返回运行模式
func (c *Config) Mode() string {
	if c.APIKey == "" {
		return "passive"
	}
	return "relay"
}

// Load 加载配置
func Load() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("无法获取用户目录: %w", err)
	}

	dataDir := filepath.Join(homeDir, ".ticktoken")
	if envDir := os.Getenv("TICKTOKEN_DATA_DIR"); envDir != "" {
		dataDir = envDir
	}

	// 确保数据目录存在
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("无法创建数据目录: %w", err)
	}

	cfg := &Config{
		ProxyAddr:     "127.0.0.1:8899",
		DashboardAddr: "127.0.0.1:8900",
		DataDir:       dataDir,
		DBPath:        filepath.Join(dataDir, "ticktoken.db"),
		CADir:         filepath.Join(dataDir, "ca"),
		APIKey:        os.Getenv("TICKTOKEN_API_KEY"),
		Verbose:       os.Getenv("TICKTOKEN_VERBOSE") == "1",
	}

	return cfg, nil
}

// Default 返回默认配置（用于 Load 失败时的降级）
// 使用当前工作目录下的临时数据目录，保证应用能启动
func Default() *Config {
	dataDir := filepath.Join(os.TempDir(), "ticktoken")
	_ = os.MkdirAll(dataDir, 0755)
	return &Config{
		ProxyAddr:     "127.0.0.1:8899",
		DashboardAddr: "127.0.0.1:8900",
		DataDir:       dataDir,
		DBPath:        filepath.Join(dataDir, "ticktoken.db"),
		CADir:         filepath.Join(dataDir, "ca"),
		APIKey:        "",
		Verbose:       false,
	}
}
