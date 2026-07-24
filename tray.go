package main

import (
	"context"
	"embed"
	"sync"
	"sync/atomic"

	"github.com/energye/systray"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed assets/tray_off.ico assets/tray_on.ico
var trayIcons embed.FS

var (
	iconOff []byte // 连发停止
	iconOn  []byte // 连发中
)

func init() {
	var err error
	iconOff, err = trayIcons.ReadFile("assets/tray_off.ico")
	if err != nil {
		panic(err)
	}
	iconOn, err = trayIcons.ReadFile("assets/tray_on.ico")
	if err != nil {
		panic(err)
	}
}

// TrayActions 托盘对应用层的回调，避免 TrayController 依赖 *App。
type TrayActions struct {
	ShowWindow func()
	HideWindow func()
	StartBurst func()
	StopBurst  func()
	QuitApp    func()
	IsEnabled  func() bool
}

// TrayController 系统托盘：关窗口进后台、图标区分启停、菜单启停/退出。
type TrayController struct {
	actions TrayActions
	ctx     context.Context

	mu       sync.Mutex
	ready    bool
	mShow    *systray.MenuItem
	mStart   *systray.MenuItem
	mStop    *systray.MenuItem
	mQuit    *systray.MenuItem
	quitting atomic.Bool // true 时允许窗口真正关闭
}

func NewTrayController() *TrayController {
	return &TrayController{}
}

func (t *TrayController) SetContext(ctx context.Context) {
	t.ctx = ctx
}

func (t *TrayController) SetActions(actions TrayActions) {
	t.mu.Lock()
	t.actions = actions
	t.mu.Unlock()
}

func (t *TrayController) Start() {
	go systray.Run(t.onReady, t.onExit)
}

func (t *TrayController) actionsSnapshot() TrayActions {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.actions
}

func (t *TrayController) onReady() {
	systray.SetIcon(iconOff)
	systray.SetTitle("K-Autokey")
	t.applyTooltip(false)

	systray.SetOnClick(func(menu systray.IMenu) {
		t.showWindow()
	})
	systray.SetOnRClick(func(menu systray.IMenu) {
		if menu != nil {
			_ = menu.ShowMenu()
		}
	})
	systray.SetOnDClick(func(menu systray.IMenu) {
		t.showWindow()
	})

	t.mShow = systray.AddMenuItem("显示主窗口", "打开 K-Autokey")
	systray.AddSeparator()
	t.mStart = systray.AddMenuItem("开启连发", "开启键盘连发")
	t.mStop = systray.AddMenuItem("关闭连发", "关闭键盘连发")
	systray.AddSeparator()
	t.mQuit = systray.AddMenuItem("退出", "完全退出程序")

	t.mShow.Click(func() { t.showWindow() })
	t.mStart.Click(func() {
		if a := t.actionsSnapshot(); a.StartBurst != nil {
			a.StartBurst()
		}
	})
	t.mStop.Click(func() {
		if a := t.actionsSnapshot(); a.StopBurst != nil {
			a.StopBurst()
		}
	})
	t.mQuit.Click(func() { t.quitApp() })

	t.mu.Lock()
	t.ready = true
	t.mu.Unlock()

	if a := t.actionsSnapshot(); a.IsEnabled != nil {
		t.UpdateEnabled(a.IsEnabled())
	}
}

func (t *TrayController) Stop() {
	t.quitting.Store(true)
	systray.Quit()
}

func (t *TrayController) onExit() {}

func (t *TrayController) showWindow() {
	if a := t.actionsSnapshot(); a.ShowWindow != nil {
		a.ShowWindow()
		return
	}
	if t.ctx == nil {
		return
	}
	runtime.WindowShow(t.ctx)
	runtime.WindowUnminimise(t.ctx)
}

// HideToTray 隐藏主窗口到后台（托盘继续运行）。
func (t *TrayController) HideToTray() {
	if a := t.actionsSnapshot(); a.HideWindow != nil {
		a.HideWindow()
		return
	}
	if t.ctx != nil {
		runtime.WindowHide(t.ctx)
	}
}

func (t *TrayController) quitApp() {
	t.quitting.Store(true)
	if a := t.actionsSnapshot(); a.QuitApp != nil {
		a.QuitApp()
		return
	}
	if t.ctx != nil {
		runtime.Quit(t.ctx)
	}
	systray.Quit()
}

// ShouldPreventClose 关闭窗口时拦截并托盘化；真正退出时放行。
func (t *TrayController) ShouldPreventClose() bool {
	if t.quitting.Load() {
		return false
	}
	t.HideToTray()
	return true
}

func (t *TrayController) applyTooltip(enabled bool) {
	if enabled {
		systray.SetTooltip("K-Autokey — 连发中")
	} else {
		systray.SetTooltip("K-Autokey — 已停止")
	}
}

// UpdateEnabled 根据连发状态切换托盘图标与提示。
func (t *TrayController) UpdateEnabled(enabled bool) {
	t.mu.Lock()
	ready := t.ready
	t.mu.Unlock()
	if !ready {
		return
	}
	if enabled {
		systray.SetIcon(iconOn)
	} else {
		systray.SetIcon(iconOff)
	}
	t.applyTooltip(enabled)
}
