.PHONY: build build-linux build-macos build-windows package-linux clean dev

# Wails 路径（从 PATH 中查找，回退到 GOPATH/bin）
WAILS ?= $(shell command -v wails 2>/dev/null || echo "$(shell go env GOPATH)/bin/wails")

# 版本号
VERSION ?= 1.0.0

# 构建桌面应用（当前平台）
build:
	$(WAILS) build

# Linux 构建
build-linux:
	$(WAILS) build -platform linux/amd64

# macOS 构建（需在 macOS 上执行）
build-macos:
	$(WAILS) build -platform darwin/universal

# Windows 构建（需在 Windows 上执行，生成 NSIS 安装包）
build-windows:
	$(WAILS) build -platform windows/amd64 -nsis

# 打包 Linux .deb
package-linux: build-linux
	@mkdir -p build/deb/DEBIAN
	@mkdir -p build/deb/usr/local/bin
	@mkdir -p build/deb/usr/share/applications
	@mkdir -p build/deb/usr/share/icons/hicolor/512x512/apps
	@cp build/bin/TickToken build/deb/usr/local/bin/ticktoken
	@cp build/appicon.png build/deb/usr/share/icons/hicolor/512x512/apps/ticktoken.png
	@echo "Package: ticktoken\nVersion: $(VERSION)\nArchitecture: amd64\nMaintainer: TickToken\nDescription: Passive Token Counter - MITM proxy based token usage tracker\nSection: utils\nPriority: optional" > build/deb/DEBIAN/control
	@echo "[Desktop Entry]\nName=TickToken\nComment=Passive Token Counter\nExec=/usr/local/bin/ticktoken\nIcon=ticktoken\nType=Application\nCategories=Utility;" > build/deb/usr/share/applications/ticktoken.desktop
	@dpkg-deb --build build/deb build/bin/ticktoken_$(VERSION)_amd64.deb
	@echo "Created: build/bin/ticktoken_$(VERSION)_amd64.deb"

# 开发模式（热重载）
dev:
	$(WAILS) dev

# 清理构建产物
clean:
	rm -rf build/bin build/deb
