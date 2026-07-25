# 前端 ↔ Go 协议

## 行为摘要

1. 绑定若干键，再开启连发（按钮、热键或进程前台自动）。
2. 按住哪个已绑键就只连发哪个；可同时按住多个。
3. 紧急停止 /「关闭」→ 停止；有进程绑定时进入 `autoPaused`，直到目标离开前台。

## 方法

| 方法 | 说明 |
|------|------|
| `GetBootstrap()` | 键位、配置、`input_status`、进程列表、状态文案 |
| `GetInputStatus()` | 当前发送/吞键状态文案 |
| `Configure(cfg)` | 推送配置（不写盘） |
| `SaveConfig(cfg)` | 推送并写入 `%APPDATA%\K-Autokey\config.json` |
| `Start()` / `Stop()` | 手动启停；有进程绑定时仅目标前台才真正开启 |
| `SetHotkeyListening(bool)` | 捕获热键时暂停全局热键边沿 |
| `ListProcesses()` | 有可见窗口的进程列表 |
| `HideToTray()` | 隐藏到托盘 |

## 配置字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `key_labels` | `string[]` | 连发键，如 `["A","Space"]` |
| `interval_ms` | `int` | 间隔 1–10000 |
| `enable_hotkey` | `string` | 开启热键，如 `f6`、`ctrl+shift+f6` |
| `emergency_hotkey` | `string` | 紧急停止热键 |
| `bound_process` | `string` | 进程名（小写）；空=不绑定 |
| `suppress_physical` | `bool` | 是否 LL 钩子吞物理键（AHK `$`）；false=透传（`~`） |

## InputStatus

```json
{
  "active": "sendinput",
  "message": "用户态 Event（keybd_event vk=0xFF + 扫描码）；已吞物理键（$）"
}
```

## 返回值

```json
{ "ok": true, "message": "...", "enabled": false }
```

`enabled` 表示连发是否真正开启。

## 事件

`EventsOn("running", enabled bool)` — 连发开关变化。

## 进程绑定

- 按进程映像名绑定（非 PID）。
- 前台匹配 → `SetEnabled(true)`（未 `autoPaused`）。
- 切走或进程退出 → `SetEnabled(false)`。
