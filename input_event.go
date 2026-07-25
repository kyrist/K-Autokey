package main

import (
	"syscall"
)

// input_event.go：热键检测用的 Win32 输入 API。
//
// 连发的发送与物理键检测已全部交给 AHK 子进程（ahk/burst.ahk），
// Go 侧只需要检测全局热键（F6/F8 等）是否按下，用 GetAsyncKeyState 即可。

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	procGetAsyncKeyState = user32.NewProc("GetAsyncKeyState")
)

// asyncKeyDown 读 GetAsyncKeyState，最高位为 1 表示当前按下。
// 供 hotkey.go 的 ParsedHotkey.IsDown 使用。
func asyncKeyDown(vk uint16) bool {
	r, _, _ := procGetAsyncKeyState.Call(uintptr(vk))
	return (r & 0x8000) != 0
}
