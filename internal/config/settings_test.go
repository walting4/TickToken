package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsValidLanguage(t *testing.T) {
	if !IsValidLanguage("zh") {
		t.Error("zh 应为有效语言")
	}
	if !IsValidLanguage("en") {
		t.Error("en 应为有效语言")
	}
	if IsValidLanguage("fr") {
		t.Error("fr 不应被支持")
	}
	if IsValidLanguage("") {
		t.Error("空字符串不应有效")
	}
}

func TestSettingsPersistence(t *testing.T) {
	tmpDir := t.TempDir()

	// 初始化设置（默认中文）
	if err := InitSettings(tmpDir); err != nil {
		t.Fatalf("InitSettings 失败: %v", err)
	}

	// 默认应为 zh
	s := GetSettings()
	if s.Language != "zh" {
		t.Errorf("默认语言应为 zh，得到 %s", s.Language)
	}

	// 更新为 en
	if err := UpdateSettings(Settings{Language: "en"}); err != nil {
		t.Fatalf("UpdateSettings 失败: %v", err)
	}

	s = GetSettings()
	if s.Language != "en" {
		t.Errorf("更新后语言应为 en，得到 %s", s.Language)
	}

	// 验证文件已写入
	data, err := os.ReadFile(filepath.Join(tmpDir, "settings.json"))
	if err != nil {
		t.Fatalf("读取 settings.json 失败: %v", err)
	}
	if !contains(string(data), "en") {
		t.Errorf("settings.json 内容应包含 en，实际: %s", string(data))
	}
}

func TestSettingsReloadFromDisk(t *testing.T) {
	tmpDir := t.TempDir()

	// 先写入一个 en 的 settings.json
	content := `{"language":"en"}`
	if err := os.WriteFile(filepath.Join(tmpDir, "settings.json"), []byte(content), 0644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}

	// 初始化时应加载 en
	if err := InitSettings(tmpDir); err != nil {
		t.Fatalf("InitSettings 失败: %v", err)
	}
	s := GetSettings()
	if s.Language != "en" {
		t.Errorf("应从磁盘加载 en，得到 %s", s.Language)
	}
}

func TestSettingsInvalidLanguageFallback(t *testing.T) {
	tmpDir := t.TempDir()

	// 写入无效语言代码
	content := `{"language":"xyz"}`
	if err := os.WriteFile(filepath.Join(tmpDir, "settings.json"), []byte(content), 0644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}

	if err := InitSettings(tmpDir); err != nil {
		t.Fatalf("InitSettings 失败: %v", err)
	}
	// 无效语言应回退到 zh
	s := GetSettings()
	if s.Language != "zh" {
		t.Errorf("无效语言应回退到 zh，得到 %s", s.Language)
	}
}

func TestSettingsCorruptedFile(t *testing.T) {
	tmpDir := t.TempDir()

	// 写入损坏 JSON
	content := `{not valid json`
	if err := os.WriteFile(filepath.Join(tmpDir, "settings.json"), []byte(content), 0644); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}

	// 损坏文件不应导致 InitSettings 失败，应使用默认值
	if err := InitSettings(tmpDir); err != nil {
		t.Fatalf("损坏文件不应导致 InitSettings 失败: %v", err)
	}
	s := GetSettings()
	if s.Language != "zh" {
		t.Errorf("损坏文件应回退到默认 zh，得到 %s", s.Language)
	}
}

func TestUpdateSettingsInvalidLanguage(t *testing.T) {
	tmpDir := t.TempDir()
	if err := InitSettings(tmpDir); err != nil {
		t.Fatalf("InitSettings 失败: %v", err)
	}

	// 尝试更新为无效语言
	if err := UpdateSettings(Settings{Language: "fr"}); err != nil {
		t.Fatalf("UpdateSettings 不应返回错误（内部回退）: %v", err)
	}
	// 应被回退到 zh
	s := GetSettings()
	if s.Language != "zh" {
		t.Errorf("无效语言应回退到 zh，得到 %s", s.Language)
	}
}

func TestGetSettingsWithoutInit(t *testing.T) {
	// 重置全局状态模拟未初始化
	origMgr := settingsMgr
	settingsMgr = nil
	defer func() { settingsMgr = origMgr }()

	s := GetSettings()
	if s.Language != "zh" {
		t.Errorf("未初始化时应返回默认 zh，得到 %s", s.Language)
	}
}

func TestSupportedLanguages(t *testing.T) {
	if len(SupportedLanguages) != 2 {
		t.Errorf("应支持 2 种语言，得到 %d", len(SupportedLanguages))
	}
	found := map[string]bool{}
	for _, l := range SupportedLanguages {
		found[l] = true
	}
	if !found["zh"] || !found["en"] {
		t.Error("SupportedLanguages 应包含 zh 和 en")
	}
}

func TestSettingsConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	if err := InitSettings(tmpDir); err != nil {
		t.Fatalf("InitSettings 失败: %v", err)
	}

	// 并发读写测试
	done := make(chan bool, 2)
	go func() {
		defer func() { done <- true }()
		for i := 0; i < 100; i++ {
			lang := "zh"
			if i%2 == 0 {
				lang = "en"
			}
			_ = UpdateSettings(Settings{Language: lang})
		}
	}()
	go func() {
		defer func() { done <- true }()
		for i := 0; i < 100; i++ {
			_ = GetSettings()
		}
	}()
	<-done
	<-done
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
