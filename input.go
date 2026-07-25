package main

// input.go：输入相关状态与热键检测入口。
//
// 连发核心由 AHK 子进程实现，Go 侧不再发送任何合成输入、不维护物理键状态、
// 不安装吞键钩子。本文件仅保留：
//   - hotkeyKeyDown：热键按下检测（供 hotkey.go 调用）
//   - InputStatus / GetInputStatus：前端展示当前输入后端说明

// InputStatus 供前端展示当前输入后端状态。
type InputStatus struct {
	Active  string `json:"active"`
	Message string `json:"message"`
}

func GetInputStatus() InputStatus {
	return InputStatus{
		Active:  "ahk",
		Message: "AutoHotkey 子进程（InstallKeybdHook + GetKeyState + SendEvent vkFFscXX）",
	}
}

// hotkeyKeyDown 热键专用按键检测：直接读 GetAsyncKeyState。
func hotkeyKeyDown(vk uint16) bool {
	return asyncKeyDown(vk)
}
