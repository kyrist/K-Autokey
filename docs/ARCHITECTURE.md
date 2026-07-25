# 架构说明

## 分层

| 层 | 位置 | 职责 |
|----|------|------|
| UI | `frontend/` | 键盘绑定、热键捕获、进程选择、吞键开关 |
| Shell | `main.go`、`app.go` | Wails 生命周期、JS↔Go、组装子系统 |
| Core | 其余 `*.go` | 连发、热键、进程、托盘、输入、配置 |

```text
frontend/app.js
      │  window.go.main.App.*
      │  EventsOn("running")
      ▼
app.go                      组装：applyConfig 分发配置
      ├── engine.go              连发引擎（按住 → 间隔脉冲）
      ├── hotkey_watch.go        全局热键
      ├── process_watch.go       前台进程自动启停
      ├── tray.go                系统托盘
      ├── hotkey.go / keys.go    热键与键位表
      ├── input.go               发送/吞键路由
      ├── input_event.go         keybd_event(vk=0xFF + 扫描码) 发送
      ├── keyboard_hook.go       WH_KEYBOARD_LL 吞物理键
      ├── physical_keys.go       物理按下状态（钩子维护）
      ├── process.go             进程枚举
      └── config.go              %APPDATA% 配置
```

## 输入路径

单一用户态路径（对齐 DNFAutoFire 的 `SendEvent vkFFscXX`）：

- **发送**：`keybd_event(vk=0xFF, 扫描码)`。`vk=0xFF` 是无效虚拟键，DNF 按扫描码响应触发连发，不影响聊天框打字；`keybd_event` 走 OS 输入队列，有背压，不会堆积阻塞回车。
- **吞键**：`WH_KEYBOARD_LL` 低级钩子（对齐 AHK `$`），连发开启时吞掉已绑键的物理事件，避免物理按下与脉冲叠加。注入事件带 `LLMHF_INJECTED` 标志，钩子一律放行。
- **物理按下检测**：钩子激活时读 `physDown`（钩子维护，带心跳看门狗）；否则读 `GetAsyncKeyState`。

定时：`timeBeginPeriod(1)` + 尾段自旋，尽量稳住间隔。`applyBurstInputMode` 统一启停吞键。

## 解耦原则

1. **Engine** 只做「按住已绑键 → 间隔脉冲」，不感知热键与进程。
2. **HotkeyWatcher / ProcessFocusWatcher** 通过窄接口操作 Engine。
3. **Tray** 只依赖 `TrayActions`，不持有 `*App`。
4. **App** 是唯一组装点。

## 状态流

```text
用户 / 热键 / 进程前台变化
        │
        ▼
   Engine.SetEnabled → applyBurstInputMode
        │
        ├── EventsEmit("running") → 前端
        └── tray.UpdateEnabled
```

- `autoPaused`：紧急停止或手动关闭后，禁止进程监视器立刻再自动打开。
- 关窗口：隐藏到托盘；托盘「退出」才结束进程。

## 打包

`.\scripts\build_app.ps1` → `build\bin\K-Autokey.exe`（需 `-tags production,desktop`）。

## 已知限制

- 依赖 WebView2
- 部分游戏/反作弊仍可能拦截用户态注入输入
