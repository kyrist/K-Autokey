package main

import (
	"syscall"
	"unsafe"
)

// input.go：Win32 键盘注入与异步按键状态（供 Engine / 热键检测使用）。
//
// - keyTap / sendKeyboard → user32.SendInput
// - asyncKeyDown → user32.GetAsyncKeyState

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	procSendInput        = user32.NewProc("SendInput")
	procGetAsyncKeyState = user32.NewProc("GetAsyncKeyState")
)

const (
	inputKeyboard        = 1
	keyeventfKeyup       = 0x0002
	keyeventfExtendedKey = 0x0001
)

type keybdInput struct {
	wVk         uint16
	wScan       uint16
	dwFlags     uint32
	time        uint32
	_           uint32
	dwExtraInfo uintptr
}

type inputKeyMsg struct {
	inputType uint32
	_         uint32
	ki        keybdInput
	_pad      [8]byte
}

func sendKeyboard(vk uint16, keyUp bool) {
	flags := uint32(0)
	if keyUp {
		flags = keyeventfKeyup
	}
	if isExtended(vk) {
		flags |= keyeventfExtendedKey
	}
	in := inputKeyMsg{
		inputType: inputKeyboard,
		ki: keybdInput{
			wVk:     vk,
			dwFlags: flags,
		},
	}
	_, _, _ = procSendInput.Call(1, uintptr(unsafe.Pointer(&in)), unsafe.Sizeof(in))
}

func keyTap(vk uint16) {
	sendKeyboard(vk, false)
	sendKeyboard(vk, true)
}

func isExtended(vk uint16) bool {
	switch vk {
	case 0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x27, 0x28, 0x2D, 0x2E,
		0x5B, 0x5C, 0x5D, 0xA3, 0xA5:
		return true
	default:
		return false
	}
}

func asyncKeyDown(vk uint16) bool {
	r, _, _ := procGetAsyncKeyState.Call(uintptr(vk))
	return (r & 0x8000) != 0
}
