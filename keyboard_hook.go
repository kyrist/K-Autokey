package main

import (
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

// keyboard_hook.go：低级键盘钩子，对齐 AHK 的 $ 热键。
//
// 连发开启时吞掉已绑键的物理事件；引擎再发送脉冲。
// 自身注入用自定义 dwExtraInfo 标记，钩子一律放行。

const (
	whKeyboardLL  = 13
	wmKeyDown     = 0x0100
	wmKeyUp        = 0x0101
	wmSysKeyDown  = 0x0104
	wmSysKeyUp    = 0x0105
	wmQuit         = 0x0012
	llkhfInjected = 0x00000010
)

type kbdLLHookStruct struct {
	vkCode      uint32
	scanCode    uint32
	flags       uint32
	time        uint32
	dwExtraInfo uintptr
}

type winMSG struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ x, y int32 }
}

var (
	procSetWindowsHookExW   = user32.NewProc("SetWindowsHookExW")
	procUnhookWindowsHookEx = user32.NewProc("UnhookWindowsHookEx")
	procCallNextHookEx      = user32.NewProc("CallNextHookEx")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessageW    = user32.NewProc("DispatchMessageW")
	procPostThreadMessageW  = user32.NewProc("PostThreadMessageW")

	hookMu       sync.Mutex
	hookRunning  bool
	hookHandle   uintptr
	hookCallback uintptr
	hookThreadID uint32
	hookReady    atomic.Bool
	hookFailed   atomic.Bool

	suppressMu  sync.RWMutex
	suppressVKs map[uint16]struct{}
	suppressOn  atomic.Bool
)

func init() {
	suppressVKs = map[uint16]struct{}{}
	hookCallback = syscall.NewCallback(lowLevelKeyboardProc)
}

func setBurstSuppress(enabled bool, vks []uint16) {
	suppressMu.Lock()
	suppressVKs = map[uint16]struct{}{}
	for _, vk := range vks {
		if vk != 0 {
			suppressVKs[vk] = struct{}{}
		}
	}
	suppressMu.Unlock()
	suppressOn.Store(enabled && len(vks) > 0)
	if !enabled {
		clearPhysicalKeys()
	}
}

func shouldSuppressVK(vk uint16) bool {
	if !suppressOn.Load() {
		return false
	}
	suppressMu.RLock()
	_, ok := suppressVKs[vk]
	suppressMu.RUnlock()
	return ok
}

var (
	hookCallCount atomic.Int32
)

func lowLevelKeyboardProc(nCode, wParam, lParam uintptr) uintptr {
	if c := hookCallCount.Add(1); c <= 10 {
		var vk uint32
		if lParam != 0 {
			kbd := (*kbdLLHookStruct)(unsafe.Pointer(lParam))
			vk = kbd.vkCode
		}
		debugLog("hook ANY call#%d nCode=%d wParam=0x%X vk=0x%X", c, int32(nCode), wParam, vk)
	}
	if int(nCode) >= 0 && lParam != 0 {
		kbd := (*kbdLLHookStruct)(unsafe.Pointer(lParam))
		// 本进程用 keybd_event 注入的事件带 llkhfInjected 标志，一律放行不吞。
		ours := (kbd.flags & llkhfInjected) != 0
		vk := uint16(kbd.vkCode)
		sup := !ours && shouldSuppressVK(vk)
		if sup {
			debugLog("hook SWALLOW vk=0x%X scan=0x%X wParam=0x%X ours=%v suppressOn=%v",
				vk, kbd.scanCode, wParam, ours, suppressOn.Load())
			switch wParam {
			case wmKeyDown, wmSysKeyDown:
				setPhysicalKey(vk, true)
			case wmKeyUp, wmSysKeyUp:
				setPhysicalKey(vk, false)
			}
			return 1
		}
	}
	r, _, _ := procCallNextHookEx.Call(0, nCode, wParam, lParam)
	return r
}

func ensureKeyHook() bool {
	return hookReady.Load()
}

func waitHookReady(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if hookReady.Load() {
			return true
		}
		if hookFailed.Load() {
			return false
		}
		time.Sleep(10 * time.Millisecond)
	}
	return hookReady.Load()
}

func keyHookReady() bool { return hookReady.Load() }

// installKeyHookOnCurrentThread 在调用线程安装 WH_KEYBOARD_LL 钩子。
// 需要该线程有消息泵（Wails 主线程 GUI 消息泵会处理钩子回调）。
func installKeyHookOnCurrentThread() bool {
	hookMu.Lock()
	if hookReady.Load() {
		hookMu.Unlock()
		return true
	}
	hookThreadID = windowsGetCurrentThreadID()
	h, _, _ := procSetWindowsHookExW.Call(
		uintptr(whKeyboardLL),
		hookCallback,
		0,
		0,
	)
	if h == 0 {
		hookThreadID = 0
		hookFailed.Store(true)
		hookMu.Unlock()
		debugLog("installKeyHookOnCurrentThread FAILED tid=%d", hookThreadID)
		return false
	}
	hookHandle = h
	hookRunning = true
	hookReady.Store(true)
	hookMu.Unlock()
	debugLog("installKeyHookOnCurrentThread OK handle=0x%X tid=%d", h, hookThreadID)
	return true
}

func stopKeyHook() {
	hookMu.Lock()
	if hookHandle != 0 {
		_, _, _ = procUnhookWindowsHookEx.Call(hookHandle)
		hookHandle = 0
	}
	hookRunning = false
	hookThreadID = 0
	hookReady.Store(false)
	hookMu.Unlock()
	clearPhysicalKeys()
	suppressOn.Store(false)
}

func windowsGetCurrentThreadID() uint32 {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	p := kernel32.NewProc("GetCurrentThreadId")
	id, _, _ := p.Call()
	return uint32(id)
}
