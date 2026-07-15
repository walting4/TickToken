package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// SupportedLanguages 支持的语言列表
var SupportedLanguages = []string{"zh", "en"}

// IsValidLanguage 校验语言代码是否受支持
func IsValidLanguage(lang string) bool {
	for _, l := range SupportedLanguages {
		if l == lang {
			return true
		}
	}
	return false
}

// Settings 用户可变设置（持久化到 settings.json）
type Settings struct {
	Language string `json:"language"` // "zh" 或 "en"，默认 "zh"
}

// settingsManager 设置管理器（线程安全）
type settingsManager struct {
	mu       sync.RWMutex
	filePath string
	current  Settings
	loaded   bool
}

var settingsMgr *settingsManager

// InitSettings 初始化设置管理器，绑定配置文件路径
// 必须在 Load 之后调用
func InitSettings(dataDir string) error {
	path := filepath.Join(dataDir, "settings.json")
	settingsMgr = &settingsManager{
		filePath: path,
		current:  Settings{Language: "zh"}, // 默认中文
	}
	return settingsMgr.load()
}

func (m *settingsManager) load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			m.loaded = true
			return nil // 文件不存在时使用默认值
		}
		return err
	}

	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		// 解析失败时使用默认值，不阻断启动
		m.loaded = true
		return nil
	}
	if !IsValidLanguage(s.Language) {
		s.Language = "zh"
	}
	m.current = s
	m.loaded = true
	return nil
}

// GetSettings 获取当前设置快照
func GetSettings() Settings {
	if settingsMgr == nil {
		return Settings{Language: "zh"}
	}
	settingsMgr.mu.RLock()
	defer settingsMgr.mu.RUnlock()
	return settingsMgr.current
}

// UpdateSettings 更新设置并持久化
func UpdateSettings(s Settings) error {
	if settingsMgr == nil {
		return nil
	}
	if !IsValidLanguage(s.Language) {
		s.Language = "zh"
	}
	settingsMgr.mu.Lock()
	defer settingsMgr.mu.Unlock()

	settingsMgr.current = s
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(settingsMgr.filePath, data, 0644)
}
