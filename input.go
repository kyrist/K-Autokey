package main

// input.go：热键检测入口。
//
// 连发核心由 AHK 子进程实现，Go 侧不再发送任何合成输入、不维护物理键状态、
// 不安装吞键钩子。本文件仅保留 hotkeyKeyDown：热键按下检测（供 hotkey.go 调用）。

// hotkeyKeyDown 热键专用按键检测：直接读 GetAsyncKeyState。
func hotkeyKeyDown(vk uint16) bool {
	return asyncKeyDown(vk)
}
