package main

import (
	"sync/atomic"
	"syscall"
)

// input_event.go：用户态回退路径（对齐 AHK SendEvent / keybd_event）。
// 配置值仍为 "sendinput"（兼容旧 config.json），实际不走 Win32 SendInput。

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	procKeybdEvent       = user32.NewProc("keybd_event")
	procGetAsyncKeyState = user32.NewProc("GetAsyncKeyState")
	procMapVirtualKeyW   = user32.NewProc("MapVirtualKeyW")
)

const (
	keyeventfKeyup       = 0x0002
	keyeventfExtendedKey = 0x0001
	mapvkVkToVsc         = 0
	mapvkVkToVscEx       = 4
)

func vkToScanEx(vk uint16) (scan uint16, extended bool) {
	r, _, _ := procMapVirtualKeyW.Call(uintptr(vk), mapvkVkToVscEx)
	if r == 0 {
		r, _, _ = procMapVirtualKeyW.Call(uintptr(vk), mapvkVkToVsc)
	}
	scan = uint16(r & 0xff)
	extended = (r&0xff00) != 0 || isExtended(vk)
	return scan, extended
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

// sendOurKey 对齐 DNFAutoFire：keybd_event(vk=0xFF, LOBYTE(sc), KEYUP|EXTENDED, 0)。
// vk=0xFF 是无效虚拟键，DNF 按扫描码响应，不影响聊天框打字；
// keybd_event 走 OS 输入队列，有背压，不会堆积阻塞回车。
func sendOurKey(vk uint16, keyUp bool) {
	scan, extended := vkToScanEx(vk)
	flags := uint32(0)
	if keyUp {
		flags |= keyeventfKeyup
	}
	if extended {
		flags |= keyeventfExtendedKey
	}
	_, _, _ = procKeybdEvent.Call(
		0xFF,
		uintptr(scan&0xff),
		uintptr(flags),
		0,
	)
}

func sendInputKeyTap(vk uint16) {
	sendOurKey(vk, false)
	if keyTapHold > 0 {
		sleepPrecise(keyTapHold)
	}
	sendOurKey(vk, true)
}

var asyncKeyDownLogCount atomic.Int32

func sendInputAsyncKeyDown(vk uint16) bool {
	r, _, _ := procGetAsyncKeyState.Call(uintptr(vk))
	down := (r & 0x8000) != 0
	if c := asyncKeyDownLogCount.Add(1); c <= 30 {
		debugLog("GetAsyncKeyState vk=0x%X r=0x%X down=%v", vk, int32(r), down)
	}
	return down
}
