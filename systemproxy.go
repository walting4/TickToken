package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// ProxyStatus 代理与证书配置状态
type ProxyStatus struct {
	ProxyAddr       string `json:"proxyAddr"`       // TickToken 代理地址
	IsListening     bool   `json:"isListening"`     // 代理是否在监听
	SystemProxySet  bool   `json:"systemProxySet"`  // 系统代理是否已配置
	CACertInstalled bool   `json:"cacertInstalled"` // CA 证书是否已安装到信任库
	TraeProxySet    bool   `json:"traeProxySet"`    // trae 代理是否已配置
	CACertPath      string `json:"cacertPath"`      // CA 证书路径
	TraeConfigPath  string `json:"traeConfigPath"`  // trae 配置文件路径
	Platform        string `json:"platform"`        // 操作系统
	ProxyStats      struct {
		TotalRequests  int64 `json:"totalRequests"`
		TotalCaptured  int64 `json:"totalCaptured"`
		TotalErrors    int64 `json:"totalErrors"`
		TotalSSEStream int64 `json:"totalSSEStream"`
	} `json:"proxyStats"`
}

// SetupResult 一键配置结果
type SetupResult struct {
	Success  bool     `json:"success"`
	Message  string   `json:"message"`
	Steps    []string `json:"steps"`    // 执行的步骤
	Warnings []string `json:"warnings"` // 警告信息
}

// GetProxyStatus 获取代理与证书的完整状态
func (a *App) GetProxyStatus() ProxyStatus {
	status := ProxyStatus{
		Platform:   runtime.GOOS,
		CACertPath: "",
	}

	if a.cfg != nil {
		status.ProxyAddr = a.cfg.ProxyAddr
	}
	if a.certMgr != nil {
		status.CACertPath = a.certMgr.GetCACertPath()
	}
	status.IsListening = a.mitmProxy != nil

	// 检查系统代理
	status.SystemProxySet = checkSystemProxy(a.cfg.ProxyAddr)

	// 检查 CA 证书
	if a.certMgr != nil {
		status.CACertInstalled = checkCACertInstalled(a.certMgr.GetCACertPath())
	}

	// 检查 trae 代理配置
	status.TraeConfigPath, status.TraeProxySet = checkTraeProxyConfig()

	// 代理统计
	if a.mitmProxy != nil {
		total, captured, errs, sse := a.mitmProxy.GetStats()
		status.ProxyStats.TotalRequests = total
		status.ProxyStats.TotalCaptured = captured
		status.ProxyStats.TotalErrors = errs
		status.ProxyStats.TotalSSEStream = sse
	}

	return status
}

// SetupProxyAndCert 一键配置系统代理 + 安装 CA 证书 + 配置 trae 代理
// 这是最核心的用户体验优化：用户只需点一个按钮即可完成所有配置
func (a *App) SetupProxyAndCert() SetupResult {
	result := SetupResult{
		Steps:    []string{},
		Warnings: []string{},
	}

	if a.cfg == nil || a.certMgr == nil {
		result.Message = "应用未完全初始化，请重启"
		return result
	}

	proxyAddr := a.cfg.ProxyAddr
	certPath := a.certMgr.GetCACertPath()

	// 步骤 1: 安装 CA 证书到系统信任库
	if err := installCACert(certPath); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("CA 证书安装失败: %v", err))
	} else {
		result.Steps = append(result.Steps, "CA 证书已安装到系统信任库")
	}

	// 步骤 2: 配置系统代理
	if err := setSystemProxy(proxyAddr); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("系统代理设置失败: %v", err))
	} else {
		result.Steps = append(result.Steps, "系统代理已配置为 "+proxyAddr)
	}

	// 步骤 3: 配置 trae 代理
	traePath, configured := configureTraeProxy(proxyAddr)
	if configured {
		result.Steps = append(result.Steps, "trae 代理已配置: "+traePath)
	} else if traePath != "" {
		result.Warnings = append(result.Warnings, "trae 代理配置失败: "+traePath)
	} else {
		result.Warnings = append(result.Warnings, "未找到 trae 配置目录，请手动在 trae 设置中配置 http.proxy")
	}

	// 步骤 4: 配置 VSCode 代理（trae 基于 VSCode，配置路径可能不同）
	if path, ok := configureVSCodeProxy(proxyAddr); ok {
		result.Steps = append(result.Steps, "VSCode 代理已配置: "+path)
	}

	result.Success = len(result.Steps) >= 2
	if result.Success {
		result.Message = "配置完成！请重启 trae/VSCode 使代理生效"
	} else {
		result.Message = "部分配置失败，请查看警告信息并手动配置"
	}

	return result
}

// RemoveProxyAndCert 一键移除系统代理配置（恢复原状）
func (a *App) RemoveProxyAndCert() SetupResult {
	result := SetupResult{
		Steps:    []string{},
		Warnings: []string{},
	}

	// 移除系统代理
	if err := removeSystemProxy(); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("移除系统代理失败: %v", err))
	} else {
		result.Steps = append(result.Steps, "系统代理已移除")
	}

	// 移除 trae 代理配置
	if path, ok := removeTraeProxy(); ok {
		result.Steps = append(result.Steps, "trae 代理配置已移除: "+path)
	}

	// 移除 VSCode 代理配置
	if path, ok := removeVSCodeProxy(); ok {
		result.Steps = append(result.Steps, "VSCode 代理配置已移除: "+path)
	}

	result.Success = true
	result.Message = "代理配置已移除，请重启 trae/VSCode"
	return result
}

// ==================== 系统代理 ====================

// checkSystemProxy 检查系统代理是否已设置为 TickToken
func checkSystemProxy(expectedAddr string) bool {
	switch runtime.GOOS {
	case "windows":
		// 检查注册表 HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings
		out, err := exec.Command("reg", "query",
			`HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings`,
			"/v", "ProxyServer").Output()
		if err != nil {
			return false
		}
		return strings.Contains(string(out), expectedAddr)
	case "darwin":
		// macOS: 检查 networksetup
		out, err := exec.Command("networksetup", "-getsecurewebproxy", "Wi-Fi").Output()
		if err != nil {
			return false
		}
		return strings.Contains(string(out), "127.0.0.1") || strings.Contains(string(out), "localhost")
	case "linux":
		// Linux: 检查环境变量（局限性大，但足够基本判断）
		return os.Getenv("HTTP_PROXY") != "" || os.Getenv("http_proxy") != ""
	}
	return false
}

// setSystemProxy 设置系统代理
func setSystemProxy(addr string) error {
	host, port, err := splitHostPort(addr)
	if err != nil {
		return err
	}

	switch runtime.GOOS {
	case "windows":
		// 设置注册表
		if err := exec.Command("reg", "add",
			`HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings`,
			"/v", "ProxyEnable", "/t", "REG_DWORD", "/d", "1", "/f").Run(); err != nil {
			return fmt.Errorf("设置 ProxyEnable 失败: %w", err)
		}
		proxyServer := fmt.Sprintf("%s:%s", host, port)
		if err := exec.Command("reg", "add",
			`HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings`,
			"/v", "ProxyServer", "/t", "REG_SZ", "/d", proxyServer, "/f").Run(); err != nil {
			return fmt.Errorf("设置 ProxyServer 失败: %w", err)
		}
		// 通知系统刷新代理设置
		exec.Command("rundll32", "wininet.dll", "InternetSetOptionW").Run()
		return nil

	case "darwin":
		// 获取活动网络服务
		networkSvc := getActiveNetworkService()
		// 设置 HTTP 代理
		exec.Command("networksetup", "-setwebproxy", networkSvc, host, port).Run()
		// 设置 HTTPS 代理
		exec.Command("networksetup", "-setsecurewebproxy", networkSvc, host, port).Run()
		return nil

	case "linux":
		// Linux 桌面环境差异大，设置环境变量作为基本方案
		// GNOME: gsettings set org.gnome.system.proxy mode 'manual'
		exec.Command("gsettings", "set", "org.gnome.system.proxy", "mode", "manual").Run()
		exec.Command("gsettings", "set", "org.gnome.system.proxy.http", "host", host).Run()
		exec.Command("gsettings", "set", "org.gnome.system.proxy.http", "port", port).Run()
		exec.Command("gsettings", "set", "org.gnome.system.proxy.https", "host", host).Run()
		exec.Command("gsettings", "set", "org.gnome.system.proxy.https", "port", port).Run()
		return nil
	}

	return fmt.Errorf("不支持的操作系统: %s", runtime.GOOS)
}

// removeSystemProxy 移除系统代理
func removeSystemProxy() error {
	switch runtime.GOOS {
	case "windows":
		exec.Command("reg", "add",
			`HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings`,
			"/v", "ProxyEnable", "/t", "REG_DWORD", "/d", "0", "/f").Run()
		exec.Command("rundll32", "wininet.dll", "InternetSetOptionW").Run()
		return nil
	case "darwin":
		networkSvc := getActiveNetworkService()
		exec.Command("networksetup", "-setwebproxystate", networkSvc, "off").Run()
		exec.Command("networksetup", "-setsecurewebproxystate", networkSvc, "off").Run()
		return nil
	case "linux":
		exec.Command("gsettings", "set", "org.gnome.system.proxy", "mode", "none").Run()
		return nil
	}
	return nil
}

// getActiveNetworkService 获取 macOS 活动网络服务名称
func getActiveNetworkService() string {
	out, err := exec.Command("networksetup", "-listallnetworkservices").Output()
	if err != nil {
		return "Wi-Fi" // 默认
	}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.Contains(line, "An asterisk") {
			continue
		}
		// 优先返回 Wi-Fi
		if strings.Contains(line, "Wi-Fi") {
			return line
		}
	}
	if len(lines) > 1 {
		return strings.TrimSpace(lines[1])
	}
	return "Wi-Fi"
}

// ==================== CA 证书安装 ====================

// checkCACertInstalled 检查 CA 证书是否已安装到系统信任库
func checkCACertInstalled(certPath string) bool {
	if _, err := os.Stat(certPath); err != nil {
		return false
	}

	switch runtime.GOOS {
	case "windows":
		out, err := exec.Command("certutil", "-store", "ROOT", "TickToken CA").Output()
		return err == nil && strings.Contains(string(out), "TickToken")
	case "darwin":
		out, err := exec.Command("security", "find-certificate", "-c", "TickToken CA",
			"/Library/Keychains/System.keychain").Output()
		return err == nil && len(out) > 0
	case "linux":
		_, err := os.Stat("/usr/local/share/ca-certificates/ticktoken-ca.crt")
		return err == nil
	}
	return false
}

// installCACert 安装 CA 证书到系统信任库
func installCACert(certPath string) error {
	switch runtime.GOOS {
	case "windows":
		// certutil -addstore -f "ROOT" ca.crt
		return exec.Command("certutil", "-addstore", "-f", "ROOT", certPath).Run()

	case "darwin":
		// security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain ca.crt
		return exec.Command("security", "add-trusted-cert", "-d", "-r", "trustRoot",
			"-k", "/Library/Keychains/System.keychain", certPath).Run()

	case "linux":
		// 复制到 /usr/local/share/ca-certificates/ 并更新
		dest := "/usr/local/share/ca-certificates/ticktoken-ca.crt"
		data, err := os.ReadFile(certPath)
		if err != nil {
			return err
		}
		if err := os.WriteFile(dest, data, 0644); err != nil {
			// 可能需要 sudo，尝试用 pkexec
			return fmt.Errorf("写入证书失败（可能需要 sudo 权限）: %w", err)
		}
		return exec.Command("update-ca-certificates").Run()
	}

	return fmt.Errorf("不支持的操作系统: %s", runtime.GOOS)
}

// ==================== trae / VSCode 代理配置 ====================

// checkTraeProxyConfig 检查 trae 的代理配置
// 返回 (配置文件路径, 是否已配置代理)
func checkTraeProxyConfig() (string, bool) {
	configPaths := getTraeConfigPaths()
	for _, path := range configPaths {
		if _, err := os.Stat(path); err == nil {
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			var settings map[string]interface{}
			if err := json.Unmarshal(data, &settings); err != nil {
				continue
			}
			if proxy, ok := settings["http.proxy"]; ok {
				if proxyStr, ok := proxy.(string); ok && proxyStr != "" {
					return path, true
				}
			}
			return path, false
		}
	}
	return "", false
}

// configureTraeProxy 配置 trae 的 http.proxy
// 返回 (配置文件路径, 是否成功)
func configureTraeProxy(proxyAddr string) (string, bool) {
	configPaths := getTraeConfigPaths()

	for _, path := range configPaths {
		dir := filepath.Dir(path)
		// 确保目录存在
		if err := os.MkdirAll(dir, 0755); err != nil {
			continue
		}

		// 读取现有配置
		settings := make(map[string]interface{})
		if data, err := os.ReadFile(path); err == nil {
			json.Unmarshal(data, &settings)
		}

		// 设置代理
		settings["http.proxy"] = "http://" + proxyAddr
		// 关闭严格 SSL 校验（因为用的是自签名 CA）
		settings["http.proxyStrictSSL"] = false
		// 排除 localhost
		settings["http.proxyBypassList"] = []string{"localhost", "127.0.0.1"}

		data, err := json.MarshalIndent(settings, "", "  ")
		if err != nil {
			continue
		}

		if err := os.WriteFile(path, data, 0644); err != nil {
			continue
		}

		return path, true
	}

	return "", false
}

// getTraeConfigPaths 获取所有可能的 trae 配置文件路径
func getTraeConfigPaths() []string {
	var paths []string
	home, _ := os.UserHomeDir()

	switch runtime.GOOS {
	case "windows":
		appdata := os.Getenv("APPDATA")
		paths = []string{
			filepath.Join(appdata, "Trae", "User", "settings.json"),
			filepath.Join(appdata, "Trae CN", "User", "settings.json"),
			filepath.Join(home, ".trae", "settings.json"),
		}
	case "darwin":
		paths = []string{
			filepath.Join(home, "Library", "Application Support", "Trae", "User", "settings.json"),
			filepath.Join(home, "Library", "Application Support", "Trae CN", "User", "settings.json"),
			filepath.Join(home, ".trae", "settings.json"),
		}
	case "linux":
		paths = []string{
			filepath.Join(home, ".config", "Trae", "User", "settings.json"),
			filepath.Join(home, ".config", "Trae CN", "User", "settings.json"),
			filepath.Join(home, ".trae", "settings.json"),
		}
	}
	return paths
}

// removeTraeProxy 移除 trae 代理配置
func removeTraeProxy() (string, bool) {
	configPaths := getTraeConfigPaths()
	for _, path := range configPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var settings map[string]interface{}
		if err := json.Unmarshal(data, &settings); err != nil {
			continue
		}
		delete(settings, "http.proxy")
		delete(settings, "http.proxyStrictSSL")
		delete(settings, "http.proxyBypassList")

		out, err := json.MarshalIndent(settings, "", "  ")
		if err != nil {
			continue
		}
		if err := os.WriteFile(path, out, 0644); err != nil {
			continue
		}
		return path, true
	}
	return "", false
}

// configureVSCodeProxy 配置 VSCode 代理
func configureVSCodeProxy(proxyAddr string) (string, bool) {
	home, _ := os.UserHomeDir()
	var paths []string

	switch runtime.GOOS {
	case "windows":
		appdata := os.Getenv("APPDATA")
		paths = []string{filepath.Join(appdata, "Code", "User", "settings.json")}
	case "darwin":
		paths = []string{filepath.Join(home, "Library", "Application Support", "Code", "User", "settings.json")}
	case "linux":
		paths = []string{filepath.Join(home, ".config", "Code", "User", "settings.json")}
	}

	for _, path := range paths {
		dir := filepath.Dir(path)
		os.MkdirAll(dir, 0755)

		settings := make(map[string]interface{})
		if data, err := os.ReadFile(path); err == nil {
			json.Unmarshal(data, &settings)
		}

		settings["http.proxy"] = "http://" + proxyAddr
		settings["http.proxyStrictSSL"] = false

		data, err := json.MarshalIndent(settings, "", "  ")
		if err != nil {
			continue
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			continue
		}
		return path, true
	}
	return "", false
}

// removeVSCodeProxy 移除 VSCode 代理配置
func removeVSCodeProxy() (string, bool) {
	home, _ := os.UserHomeDir()
	var paths []string

	switch runtime.GOOS {
	case "windows":
		appdata := os.Getenv("APPDATA")
		paths = []string{filepath.Join(appdata, "Code", "User", "settings.json")}
	case "darwin":
		paths = []string{filepath.Join(home, "Library", "Application Support", "Code", "User", "settings.json")}
	case "linux":
		paths = []string{filepath.Join(home, ".config", "Code", "User", "settings.json")}
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var settings map[string]interface{}
		if err := json.Unmarshal(data, &settings); err != nil {
			continue
		}
		delete(settings, "http.proxy")
		delete(settings, "http.proxyStrictSSL")

		out, err := json.MarshalIndent(settings, "", "  ")
		if err != nil {
			continue
		}
		if err := os.WriteFile(path, out, 0644); err != nil {
			continue
		}
		return path, true
	}
	return "", false
}

// ==================== 工具函数 ====================

// splitHostPort 分割 host:port
func splitHostPort(addr string) (string, string, error) {
	idx := strings.LastIndex(addr, ":")
	if idx < 0 {
		return "", "", fmt.Errorf("无效的地址格式: %s", addr)
	}
	return addr[:idx], addr[idx+1:], nil
}
