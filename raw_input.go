package main

import (
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

// raw_input.go：用 Raw Input 检测物理键盘状态。
//
// DNF 的内核保护会拦截 WH_KEYBOARD_LL 钩子和 GetAsyncKeyState，
// 导致连发键在 DNF 前台无法检测物理按下。Raw Input 是内核级输入，
// 不经过 LL 钩子链，DNF 无法拦截。
//
// 创建隐藏窗口注册 Raw Input（RIDEV_INPUTSINK 即使窗口不在前台也接收），
// 在消息泵中处理 WM_INPUT，维护 physDown（复用 physical_keys.go）。

const (
	wmInput         = 0x00FF
	ridevInputSink  = 0x00000100
	ridInput        = 0x10000000
	rimTypekeyboard = 1
)

type rawInputDevice struct {
	usUsagePage uint16
	usUsage     uint16
	dwFlags     uint32
	hwndTarget  uintptr
}

type rawInputHeader struct {
	dwType  uint32
	dwSize  uint32
	hDevice uintptr
	wParam  uintptr
}

type rawKeyboard struct {
	makeCode         uint16
	flags            uint16
	reserved         uint16
	vKey             uint16
	message          uint16
	extraInformation uint16
}

type rawInput struct {
	header rawInputHeader
	keyboard rawKeyboard
	_ [16]byte // padding for union
}

var (
	procRegisterRawInputDevices = user32.NewProc("RegisterRawInputDevices")
	procGetRawInputData         = user32.NewProc("GetRawInputData")
	procCreateWindowExW         = user32.NewProc("CreateWindowExW")
	procRegisterClassExW        = user32.NewProc("RegisterClassExW")
	procDefWindowProcW          = user32.NewProc("DefWindowProcW")
	procDestroyWindow           = user32.NewProc("DestroyWindow")
	procUnregisterClassW        = user32.NewProc("UnregisterClassW")
	procGetMessageW2            = user32.NewProc("GetMessageW")
	procPostMessageW            = user32.NewProc("PostMessageW")

	rawMu       sync.Mutex
	rawRunning  bool
	rawHwnd     uintptr
	rawStopCh   chan struct{}
	rawWndProc  uintptr
	rawLogCount  int
)

func init() {
	rawWndProc = syscall.NewCallback(rawInputWndProc)
}

func rawInputWndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	if msg == wmInput {
		handleRawInput(lParam)
		_, _, _ = procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
	return r
}

func handleRawInput(hRawInput uintptr) {
	var data rawInput
	size := uint32(unsafe.Sizeof(data))
	r, _, _ := procGetRawInputData.Call(
		hRawInput,
		uintptr(ridInput),
		uintptr(unsafe.Pointer(&data)),
		uintptr(unsafe.Pointer(&size)),
		uintptr(unsafe.Sizeof(rawInputHeader{})),
	)
	if r == ^uintptr(0) || r == 0 {
		return
	}
	if data.header.dwType != rimTypekeyboard {
		return
	}
	kb := data.keyboard
	vk := kb.vKey
	if vk == 0 || vk == 0xFF {
		return
	}
	rawLogCount++
	if rawLogCount <= 20 {
		debugLog("RAW vk=0x%X msg=0x%X flags=0x%X", vk, kb.message, kb.flags)
	}
	switch kb.message {
	case 0x0100, 0x0104: // WM_KEYDOWN, WM_SYSKEYDOWN
		setPhysicalKey(vk, true)
	case 0x0101, 0x0105: // WM_KEYUP, WM_SYSKEYUP
		setPhysicalKey(vk, false)
	}
}

func startRawInput() bool {
	rawMu.Lock()
	if rawRunning {
		rawMu.Unlock()
		return true
	}
	rawStopCh = make(chan struct{})
	rawRunning = true
	rawMu.Unlock()

	go rawInputThread()
	return true
}

func stopRawInput() {
	rawMu.Lock()
	if !rawRunning {
		rawMu.Unlock()
		return
	}
	rawRunning = false
	if rawHwnd != 0 {
		_, _, _ = procPostMessageW.Call(rawHwnd, wmQuit, 0, 0)
	}
	rawMu.Unlock()
}

type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszClassName uintptr
	hIconSm       uintptr
}

func rawInputThread() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	defer func() {
		if r := recover(); r != nil {
			debugLog("rawInput: PANIC %v", r)
		}
	}()
	debugLog("rawInput: thread start")

	const className = "KAutokeyRawInput"
	classNamePtr, _ := syscall.UTF16PtrFromString(className)

	hInst, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GetModuleHandleW").Call(0)

	wc := wndClassEx{
		cbSize:        uint32(unsafe.Sizeof(wndClassEx{})),
		lpfnWndProc:   rawWndProc,
		hInstance:     hInst,
		lpszClassName: uintptr(unsafe.Pointer(classNamePtr)),
	}

	clsRet, _, _ := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	debugLog("rawInput: RegisterClass ret=%d hInst=0x%X", int16(clsRet), hInst)

	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(classNamePtr)),
		0,
		0,
		0, 0, 0, 0,
		0, 0, hInst, 0,
	)
	if hwnd == 0 {
		debugLog("rawInput: CreateWindow FAILED")
		rawMu.Lock()
		rawRunning = false
		rawMu.Unlock()
		return
	}

	rawMu.Lock()
	rawHwnd = hwnd
	rawMu.Unlock()

	// 注册键盘 Raw Input
	dev := rawInputDevice{
		usUsagePage: 0x01,
		usUsage:     0x06,
		dwFlags:     ridevInputSink,
		hwndTarget:  hwnd,
	}
	ret, _, _ := procRegisterRawInputDevices.Call(
		uintptr(unsafe.Pointer(&dev)),
		1,
		uintptr(unsafe.Sizeof(dev)),
	)
	debugLog("rawInput: Register hwnd=0x%X ret=%d", hwnd, int32(ret))

	var msg winMSG
	for {
		ret, _, _ := procGetMessageW2.Call(uintptr(unsafe.Pointer(&msg)), hwnd, 0, 0)
		if int32(ret) <= 0 {
			break
		}
		_, _, _ = procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		_, _, _ = procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}

	rawMu.Lock()
	if rawHwnd != 0 {
		_, _, _ = procDestroyWindow.Call(rawHwnd)
		rawHwnd = 0
	}
	rawRunning = false
	rawMu.Unlock()
	_, _, _ = procUnregisterClassW.Call(uintptr(unsafe.Pointer(classNamePtr)), 0)
	debugLog("rawInput: thread exit")
}
