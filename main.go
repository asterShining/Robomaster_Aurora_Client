package main

import (
	"embed"
	"log"
	"os"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var icon []byte

const appWindowTitle = "RoboMaster 2026 Custom Client"

// configureLinuxWindowBackend 在 Linux 下统一 HUD 的窗口后端。
// What: Wayland 会话且同时暴露 DISPLAY 时，强制 Wails/GTK 走 x11 backend。
// Why: 当前官方视频层已经固定依赖 X11/XWayland 做绝对几何解析与窗口控层；HUD 若继续跑在原生 Wayland，就会与 ffplay 分属两套窗口系统，层级与多屏坐标都会再次失稳。
func configureLinuxWindowBackend() {
	sessionType := strings.ToLower(strings.TrimSpace(os.Getenv("XDG_SESSION_TYPE")))
	display := strings.TrimSpace(os.Getenv("DISPLAY"))
	currentBackend := strings.ToLower(strings.TrimSpace(os.Getenv("GDK_BACKEND")))

	if sessionType != "wayland" || display == "" {
		return
	}
	if currentBackend == "x11" {
		return
	}

	// What: 直接在 GTK 初始化前覆盖进程环境变量。
	// Why: 这是让 Wails 主窗稳定落到 XWayland 的最小改法，既不需要额外启动脚本，也能覆盖 dev 与 build 后的正式运行。
	if err := os.Setenv("GDK_BACKEND", "x11"); err != nil {
		log.Printf("[Warning] failed to force GDK_BACKEND=x11: %v", err)
		return
	}

	log.Printf("[window] Force Wails HUD onto X11 backend because session=%s display=%s", sessionType, display)
}

func main() {
	// What: 在 GTK/Wails 真正初始化前，先统一 HUD 所在的窗口后端。
	// Why: 一旦窗口系统已经完成初始化，再改环境变量就来不及了，层级和多屏问题会重新退回不可控状态。
	configureLinuxWindowBackend()

	// 创建生命周期管理者
	app := NewApp()

	// 启动 Wails 窗口服务器
	err := wails.Run(&options.App{
		Title:             appWindowTitle,
		Width:             1920,
		Height:            1080,
		DisableResize:     true,
		Frameless:         true,
		AlwaysOnTop:       true,
		Fullscreen:        true,
		WindowStartState:  options.Fullscreen,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		// What: 整个 Wails 窗口改为透明 HUD 叠加层。
		// Why: 真正的视频显示现在由底层 ffplay 原生窗口负责，Webview 只保留 HUD 和交互。
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 0},
		OnStartup:        app.startup,
		// What: 等 DOM 与主窗口真正就绪后再启动原生视频层。
		// Why: ffplay 若抢在 Wails 全屏置顶完成前进入 fullscreen 层级，很容易把 HUD 压到后面，导致界面直接“消失”。
		OnDomReady:       app.domReady,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
		Linux: &linux.Options{
			Icon:                icon,
			WindowIsTranslucent: true,
			WebviewGpuPolicy:    linux.WebviewGpuPolicyAlways,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
