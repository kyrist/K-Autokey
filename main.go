// Package main 为 K-Autokey Wails 桌面应用入口。
//
// 职责划分见 docs/ARCHITECTURE.md：本文件仅启动 Wails、嵌入 frontend、绑定 App。
package main

import (
	"embed"
	"io/fs"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend
var embeddedFrontend embed.FS

func main() {
	// embed 后路径为 frontend/index.html，需 Sub 成资源根目录
	assets, err := fs.Sub(embeddedFrontend, "frontend")
	if err != nil {
		log.Fatal(err)
	}

	app := NewApp()

	err = wails.Run(&options.App{
		Title:            "K-Autokey",
		Width:            920,
		Height:           640,
		MinWidth:         920,
		MinHeight:        640,
		MaxWidth:         920,
		MaxHeight:        640,
		DisableResize:    true,
		WindowStartState: options.Normal,
		Frameless:        false,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 245, G: 246, B: 248, A: 255},
		OnStartup:        app.startup,
		OnDomReady:       app.domReady,
		OnBeforeClose:    app.beforeClose, // 关闭→托盘，非退出
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisableWindowIcon:   false,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
