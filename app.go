package main

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App 是 Wails 绑定到前端的组装层。
//
// 本身不实现连发/热键/进程监视细节，只负责：
//   - 创建并启动 Engine、HotkeyWatcher、ProcessFocusWatcher、Tray
//   - 将 UIConfig / AppConfig 通过 applyConfig 分发给各子系统
//   - 向前端暴露 GetBootstrap、Start、Stop 等方法
type App struct {
	ctx       context.Context
	engine    *Engine
	hotkeys   *HotkeyWatcher
	procWatch *ProcessFocusWatcher
	tray      *TrayController
	config    AppConfig
}

func NewApp() *App {
	return &App{
		config: LoadConfig(),
		tray:   NewTrayController(),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	debugLogInit()
	debugLog("startup begin")
	enableHighResolutionTimer()
	ensurePhysWake()
	// 连发核心由 AHK 子进程实现（绕过 DNF 对 Go 进程 LL 钩子的拦截）。
	a.engine = NewEngine(a.onBurstStateChanged)
	a.hotkeys = NewHotkeyWatcher(a) // 热键走 App，与进程绑定统一受总开关控制
	a.procWatch = NewProcessFocusWatcher(a.engine)

	a.applyConfig(a.config)

	a.engine.StartWatcher()
	a.hotkeys.Start()
	a.procWatch.Start()

	a.tray.SetContext(ctx)
	a.tray.SetActions(TrayActions{
		ShowWindow: a.showMainWindow,
		HideWindow: a.hideMainWindow,
		StartBurst: func() { _ = a.Start() },
		StopBurst:  func() { _ = a.Stop() },
		QuitApp: func() {
			runtime.Quit(a.ctx)
		},
		IsEnabled: func() bool {
			return a.engine != nil && a.engine.IsArmed()
		},
	})
	a.tray.Start()
	a.tray.UpdateEnabled(a.engine.IsEnabled())
	debugLog("startup done")
}

func (a *App) onBurstStateChanged(enabled bool) {
	// 引擎状态变更的唯一出口：通知前端 + 托盘，避免子系统互相调用 UI
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "running", enabled)
		runtime.EventsEmit(a.ctx, "status", a.statusText(enabled))
	}
	if a.tray != nil {
		a.tray.UpdateEnabled(a.engine != nil && a.engine.IsArmed())
	}
}

// IsInjecting / ToggleEnabled / EmergencyStop：供 HotkeyWatcher 调用（BurstControl）。
func (a *App) IsInjecting() bool {
	return a.engine != nil && a.engine.IsInjecting()
}

func (a *App) ToggleEnabled() {
	if a.engine == nil {
		debugLog("ToggleEnabled: engine==nil")
		return
	}
	if a.engine.IsArmed() {
		debugLog("ToggleEnabled: IsArmed=true -> Stop")
		_ = a.Stop()
		return
	}
	debugLog("ToggleEnabled: IsArmed=false -> Start")
	_ = a.Start()
}

func (a *App) EmergencyStop() {
	_ = a.Stop()
}

// syncBurstWithFocus：总开关打开后，按是否绑定进程决定 enabled。
func (a *App) syncBurstWithFocus() {
	if a.engine == nil {
		return
	}
	if !a.engine.IsArmed() {
		debugLog("syncBurstWithFocus: not armed -> SetEnabled(false)")
		a.engine.SetEnabled(false)
		return
	}
	if a.procWatch != nil && a.procWatch.Bound() != "" {
		fg := a.procWatch.IsTargetForeground()
		debugLog("syncBurstWithFocus: bound=%q fg=%v -> SetEnabled(%v)", a.procWatch.Bound(), fg, fg)
		a.engine.SetEnabled(fg)
		return
	}
	debugLog("syncBurstWithFocus: no binding -> SetEnabled(true)")
	a.engine.SetEnabled(true)
}

func (a *App) showMainWindow() {
	if a.ctx == nil {
		return
	}
	runtime.WindowShow(a.ctx)
	runtime.WindowUnminimise(a.ctx)
	runtime.WindowSetAlwaysOnTop(a.ctx, true)
	runtime.WindowSetAlwaysOnTop(a.ctx, false)
}

func (a *App) hideMainWindow() {
	if a.ctx != nil {
		runtime.WindowHide(a.ctx)
	}
}

func (a *App) domReady(ctx context.Context) {
	runtime.WindowShow(ctx)
	runtime.WindowSetAlwaysOnTop(ctx, true)
	runtime.WindowSetAlwaysOnTop(ctx, false)
	runtime.WindowCenter(ctx)
}

func (a *App) shutdown(ctx context.Context) {
	if a.procWatch != nil {
		a.procWatch.Stop()
	}
	if a.hotkeys != nil {
		a.hotkeys.Stop()
	}
	if a.engine != nil {
		a.engine.EmergencyStop()
		a.engine.StopWatcher()
	}
	if a.tray != nil {
		a.tray.Stop()
	}
	disableHighResolutionTimer()
}

func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	if a.tray != nil {
		return a.tray.ShouldPreventClose()
	}
	return false
}

// HideToTray 供前端「后台」按钮调用。
func (a *App) HideToTray() ActionResult {
	if a.tray != nil {
		a.tray.HideToTray()
	}
	enabled := a.engine != nil && a.engine.IsEnabled()
	return ActionResult{OK: true, Message: "已切换到后台，托盘图标可继续操作", Enabled: enabled}
}

// Bootstrap 返回前端初始化数据。
type Bootstrap struct {
	KeyChoices  []string               `json:"key_choices"`
	Config      map[string]interface{} `json:"config"`
	Status      string                 `json:"status"`
	Enabled     bool                   `json:"enabled"`
	Processes   []ProcessInfo          `json:"processes"`
	InputStatus InputStatus            `json:"input_status"`
}

// ActionResult 供前端判断操作结果。
type ActionResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
	Enabled bool   `json:"enabled"`
}

// UIConfig 对应前端 collectConfig() 字段。
type UIConfig struct {
	KeyLabels        []string `json:"key_labels"`
	IntervalMs       int      `json:"interval_ms"`
	EnableHotkey     string   `json:"enable_hotkey"`
	EmergencyHotkey  string   `json:"emergency_hotkey"`
	BoundProcess     string   `json:"bound_process"`
	SuppressPhysical bool     `json:"suppress_physical"`
}

func (a *App) GetBootstrap() Bootstrap {
	enabled := a.engine != nil && a.engine.IsEnabled()
	return Bootstrap{
		KeyChoices:  KeyLabels(),
		Config:      a.config.ToMap(),
		Status:      a.statusText(enabled),
		Enabled:     enabled,
		Processes:   ListWindowProcesses(),
		InputStatus: GetInputStatus(),
	}
}

func (a *App) GetInputStatus() InputStatus {
	return GetInputStatus()
}

func (a *App) ListProcesses() []ProcessInfo {
	return ListWindowProcesses()
}

func (a *App) SetHotkeyListening(on bool) ActionResult {
	enabled := a.engine != nil && a.engine.IsEnabled()
	if on && enabled {
		return ActionResult{OK: false, Message: "开启连发时不可修改热键", Enabled: enabled}
	}
	if a.hotkeys != nil {
		a.hotkeys.SetListening(on)
	}
	return ActionResult{OK: true, Enabled: enabled}
}

func (a *App) Configure(cfg UIConfig) ActionResult {
	parsed := NormalizeUIConfig(cfg)
	if a.engine != nil && a.engine.IsEnabled() {
		parsed.EnableHotkey = a.config.EnableHotkey
		parsed.EmergencyHotkey = a.config.EmergencyHotkey
	}
	a.config = parsed
	a.applyConfig(parsed)
	enabled := a.engine != nil && a.engine.IsEnabled()
	return ActionResult{OK: true, Enabled: enabled}
}

func (a *App) SaveConfig(cfg UIConfig) ActionResult {
	parsed := NormalizeUIConfig(cfg)
	if a.engine != nil && a.engine.IsEnabled() {
		parsed.EnableHotkey = a.config.EnableHotkey
		parsed.EmergencyHotkey = a.config.EmergencyHotkey
	}
	a.config = parsed
	a.applyConfig(parsed)
	enabled := a.engine != nil && a.engine.IsEnabled()
	if err := SaveConfig(parsed); err != nil {
		return ActionResult{OK: false, Message: "状态：保存失败 — " + err.Error(), Enabled: enabled}
	}
	return ActionResult{OK: true, Message: "状态：配置已保存", Enabled: enabled}
}

// Start 打开总开关；有进程绑定时仅在目标前台时真正连发。
func (a *App) Start() ActionResult {
	if len(a.config.KeyLabels) == 0 {
		return ActionResult{OK: false, Message: "状态：请至少选择一个连发按键", Enabled: false}
	}
	if a.hotkeys != nil {
		a.hotkeys.SetListening(false)
	}
	a.engine.SetArmed(true)
	a.syncBurstWithFocus()

	enabled := a.engine.IsEnabled()
	msg := a.statusText(enabled)
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "status", msg)
	}
	if a.tray != nil {
		a.tray.UpdateEnabled(true)
	}
	return ActionResult{OK: true, Message: msg, Enabled: enabled}
}

func (a *App) Stop() ActionResult {
	a.engine.SetArmed(false)
	msg := "状态：已关闭"
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "running", false)
		runtime.EventsEmit(a.ctx, "status", msg)
	}
	if a.tray != nil {
		a.tray.UpdateEnabled(false)
	}
	return ActionResult{OK: true, Message: msg, Enabled: false}
}

// applyConfig 把一份配置拆开交给各独立子系统，避免它们互相读取对方状态。
func (a *App) applyConfig(cfg AppConfig) {
	SetSuppressPhysical(cfg.SuppressPhysical)
	if a.engine != nil {
		a.engine.Configure(RepeatSettings{
			KeyVKs:     LabelsToVKs(cfg.KeyLabels),
			IntervalMs: cfg.IntervalMs,
		})
		a.engine.SyncSuppressMode()
	}
	if a.hotkeys != nil {
		a.hotkeys.Configure(HotkeyBindings{
			Enable:    cfg.EnableHotkey,
			Emergency: cfg.EmergencyHotkey,
		})
	}
	if a.procWatch != nil {
		a.procWatch.SetBound(cfg.BoundProcess)
	}
}

func (a *App) statusText(enabled bool) string {
	armed := a.engine != nil && a.engine.IsArmed()
	if enabled {
		if a.config.BoundProcess != "" {
			return "状态：连发中（" + a.config.BoundProcess + " 前台）"
		}
		return "状态：已开启 — 按住已选键即可连发"
	}
	if armed && a.config.BoundProcess != "" {
		return "状态：已开启自动 — 请切到 " + a.config.BoundProcess + " 窗口（热键可关闭）"
	}
	if armed {
		return "状态：已开启 — 按住已选键即可连发"
	}
	if a.config.BoundProcess != "" {
		return "状态：已关闭 — 按热键或「开启」后，" + a.config.BoundProcess + " 前台才连发"
	}
	return "状态：未开启 — 点「开启」或按 " + FormatHotkeyDisplay(a.config.EnableHotkey)
}
