# 架构说明

## 分层

| 层 | 位置 | 职责 |
|----|------|------|
| UI | `frontend/` | 键盘绑定、热键捕获、进程选择弹窗、状态展示 |
| Shell | `main.go`、`app.go` | Wails 窗口生命周期、JS↔Go 绑定、组装子系统 |
| Core | 其余 `*.go` | 连发、热键、进程、托盘、输入、配置 |

```text
frontend/app.js
      │  window.go.main.App.*
      │  EventsOn("running")
      ▼
app.go                 组装层：applyConfig 分发配置，不写监视细节
      ├── engine.go         连发引擎（RepeatSettings）
      ├── hotkey_watch.go   全局热键 → BurstControl
      ├── process_watch.go  前台进程自动启停 → FocusBurstControl
      ├── tray.go           托盘 UI → TrayActions 回调
      ├── hotkey.go         组合键解析 / 展示 / IsDown
      ├── input.go          SendInput、GetAsyncKeyState
      ├── process.go        进程枚举、前台进程名
      ├── keys.go           UI 键位 → VK
      └── config.go         %APPDATA% 配置读写
```

## 解耦原则

1. **Engine** 只做「按住已绑键 → 间隔注入」，不感知热键与进程。
2. **HotkeyWatcher / ProcessFocusWatcher** 通过窄接口调用 Engine（`IsInjecting`、`SetEnabled` 等），不访问未导出字段。
3. **Tray** 只依赖 `TrayActions` 回调，不持有 `*App`。
4. **App** 是唯一组装点：`applyConfig` 把配置拆给引擎 / 热键 / 进程监视器。

## 状态流

```text
用户 / 热键 / 进程前台变化
        │
        ▼
   Engine.SetEnabled
        │
        ├── EventsEmit("running") → 前端
        └── tray.UpdateEnabled    → 托盘图标
```

- `autoPaused`：紧急停止或手动关闭后，禁止进程监视器立刻再自动打开；目标进程离开前台后清除。
- 关窗口：`OnBeforeClose` → 隐藏到托盘（不退出）；托盘「退出」才真正结束进程。

## 打包

`.\scripts\build_app.ps1` → `build/bin/K-Autokey.exe`  
需 `-tags production,desktop`；可选 UPX。

## 已知限制

- 依赖系统 WebView2（不内嵌完整运行时）
- 用户态 `SendInput` 可能被部分游戏/反作弊拦截
