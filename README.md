# K-Autokey

Windows 键盘连发工具（按住已绑键则按间隔连发该键）。

**架构**：Go 核心 + Wails（WebView2）嵌入 HTML 界面；关闭窗口可托盘后台运行。

```text
┌─────────────────────────────┐
│  桌面窗口 / 系统托盘          │
│   frontend/*.html/js        │
│          ↕ Bind / Events    │
│   App 组装层                 │
│   Engine · Hotkey · Process │
└─────────────────────────────┘
```

详见 [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)、[docs/PROTOCOL.md](docs/PROTOCOL.md)。

## 功能

- 可视化键盘绑定多个键；开启后按住哪个键只连发哪个键
- 可配置连发间隔、开启热键、紧急停止（支持组合键，按键捕获设置）
- **绑定进程**：目标进程在前台时自动开连发，切走或进程退出时自动关
- **系统托盘**：关窗口 /「后台」隐藏到托盘；图标区分启停；右键可显示、启停、退出
- 配置：`%APPDATA%\K-Autokey\config.json`

## 环境

| 组件 | 说明 |
|------|------|
| Windows 10/11 | 需已安装 WebView2 |
| Go 1.25+ | 开发与编译 |
| Wails CLI | 可选：`go install github.com/wailsapp/wails/v2/cmd/wails@latest` |

## 输入方式

单一用户态路径（对齐 [DNFAutoFire](https://github.com/BaiWanly/DNFAutoFire) 的 `SendEvent vkFFscXX`）：

- **发送**：`keybd_event(vk=0xFF, 扫描码)`。`vk=0xFF` 是无效虚拟键，DNF 按扫描码响应触发连发，不影响聊天框打字；`keybd_event` 走 OS 输入队列，有背压，不会堆积阻塞回车。
- **吞键**：`WH_KEYBOARD_LL` 低级钩子（对齐 AHK `$`），连发开启时吞掉已绑键的物理事件，避免物理按下与脉冲叠加。可在 UI 勾选「吞物理键 ($)」开关；不勾选则透传物理键（AHK `~`）。

部分游戏/反作弊仍可能拦截用户态注入输入。

## 开发运行

不要用裸的 `go run .` / `go build`（会弹 Wails build tags 错误框）。

```powershell
cd K-Autokey
go mod tidy

wails dev
# 或
.\scripts\dev.ps1
# 或
go run -tags "production,desktop" .
```

## 打包

```powershell
.\scripts\build_app.ps1 -FetchUPX   # 推荐：去符号 + UPX
.\scripts\build_app.ps1 -NoUPX      # 仅去符号
```

产出：`build\bin\K-Autokey.exe`（约 11 MB；UPX 后约 3 MB）。

手动编译：

```powershell
go build -tags "production,desktop" -trimpath "-ldflags=-H=windowsgui -s -w" -o .\build\bin\K-Autokey.exe .
```

注意：必须带 `-H=windowsgui`，否则会弹出黑色 cmd 窗口。`go run` 开发时仍可能有控制台，属正常现象。

## 默认热键

| 作用 | 默认 |
|------|------|
| 开启/关闭连发 | F6 |
| 紧急停止 | F8 |

存储格式：`f6`、`ctrl+shift+f6`、`lalt+a`。点击热键按钮后按组合键设置；`Esc` 取消。开启连发时不可改热键。

## 目录

```text
K-Autokey/
├── main.go / app.go          # Wails 入口与组装
├── engine.go                 # 连发引擎
├── input.go                  # 发送/吞键路由
├── input_event.go            # keybd_event(vk=0xFF + 扫描码) 发送
├── keyboard_hook.go          # WH_KEYBOARD_LL 吞物理键
├── physical_keys.go          # 物理按下状态（钩子维护）
├── hotkey*.go / process*.go  # 热键与进程
├── keys.go / config.go / tray.go
├── frontend/                 # UI
├── docs/                      # 架构与协议
└── scripts/                   # 开发 / 打包
```

## 许可与用途

本地辅助工具。请遵守目标软件服务条款与当地法律。
