# 前端 ↔ Go 协议

## 行为摘要

1. 绑定若干键（如 A、S），再开启连发（按钮、热键或进程前台自动）。
2. 按住 A → 仅连发 A；按住 S → 仅连发 S；可同时按住多个已绑键。
3. 紧急停止热键 /「关闭」→ 停止连发；有进程绑定时会进入 `autoPaused`，直到目标离开前台。

## 方法

| 方法 | 说明 |
|------|------|
| `GetBootstrap()` | 键位列表、配置、是否已开启、进程列表、状态文案 |
| `Configure(cfg)` | 推送配置到各子系统（不写盘） |
| `SaveConfig(cfg)` | 推送并写入 `%APPDATA%\K-Autokey\config.json` |
| `Start()` / `Stop()` | 手动启停；有进程绑定时仅目标前台才真正开启 |
| `SetHotkeyListening(bool)` | 捕获热键时暂停全局热键边沿触发 |
| `ListProcesses()` | 刷新有可见窗口的进程列表 |
| `HideToTray()` | 隐藏主窗口到系统托盘 |

## 配置字段（`UIConfig` / `config.json`）

| 字段 | 类型 | 说明 |
|------|------|------|
| `key_labels` | `string[]` | 连发键，如 `["A","Space"]`；左右修饰键用 `LShift`/`RCtrl` 等 |
| `interval_ms` | `int` | 连发间隔 1–10000 |
| `enable_hotkey` | `string` | 开启/切换热键，如 `f6`、`ctrl+shift+f6` |
| `emergency_hotkey` | `string` | 紧急停止热键 |
| `bound_process` | `string` | 绑定进程名（小写，如 `notepad.exe`）；空=不绑定 |

热键规范名由后端 `NormalizeHotkey` 统一；开启连发时修改热键会被忽略。

## 返回值

多数写操作返回：

```json
{ "ok": true, "message": "...", "enabled": false }
```

`enabled` 表示当前连发是否真正开启（供 UI 与事件对齐）。

## 事件

`EventsOn("running", enabled bool)` — 连发功能开关变化（不是「某键正在按下」）。

## 进程绑定约定

- 按 **进程映像名** 绑定（非 PID），重启后同名仍有效。
- 前台匹配 → 自动 `SetEnabled(true)`（未 `autoPaused` 时）。
- 切走前台或进程退出 → `SetEnabled(false)`。
