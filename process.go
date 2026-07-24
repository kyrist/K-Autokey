package main

import (
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"unsafe"
)

// process.go：Windows 进程/窗口枚举与前台进程查询。
//
// 供进程绑定 UI（ListWindowProcesses）与 ProcessFocusWatcher 使用。
// 与 input.go 共用 user32，但职责仅限进程信息，不含键盘注入。

var (
	procGetForegroundWindow        = user32.NewProc("GetForegroundWindow")
	procGetWindowThreadProcessId   = user32.NewProc("GetWindowThreadProcessId")
	procEnumWindows                = user32.NewProc("EnumWindows")
	procIsWindowVisible            = user32.NewProc("IsWindowVisible")
	procGetWindow                  = user32.NewProc("GetWindow")
	procGetWindowTextW             = user32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW       = user32.NewProc("GetWindowTextLengthW")
	kernel32                       = syscall.NewLazyDLL("kernel32.dll")
	procOpenProcess                = kernel32.NewProc("OpenProcess")
	procCloseHandle                = kernel32.NewProc("CloseHandle")
	procQueryFullProcessImageNameW = kernel32.NewProc("QueryFullProcessImageNameW")
	procGetCurrentProcessId        = kernel32.NewProc("GetCurrentProcessId")
	procCreateToolhelp32Snapshot   = kernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32FirstW            = kernel32.NewProc("Process32FirstW")
	procProcess32NextW             = kernel32.NewProc("Process32NextW")
)

const (
	processQueryLimitedInformation = 0x1000
	gwOwner                        = 4
	th32csSnapProcess              = 0x00000002
	maxPath                        = 260
)

type processEntry32W struct {
	Size            uint32
	Usage           uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	Threads         uint32
	ParentProcessID uint32
	PriClassBase    int32
	Flags           uint32
	ExeFile         [maxPath]uint16
}

// ProcessInfo 可供绑定的窗口进程。
type ProcessInfo struct {
	Name  string `json:"name"`  // 如 notepad.exe
	PID   uint32 `json:"pid"`
	Title string `json:"title"` // 窗口标题
}

func currentPID() uint32 {
	r, _, _ := procGetCurrentProcessId.Call()
	return uint32(r)
}

func foregroundPID() uint32 {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return 0
	}
	var pid uint32
	_, _, _ = procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	return pid
}

func processImageName(pid uint32) string {
	if pid == 0 {
		return ""
	}
	h, _, _ := procOpenProcess.Call(processQueryLimitedInformation, 0, uintptr(pid))
	if h == 0 {
		return ""
	}
	defer procCloseHandle.Call(h)

	buf := make([]uint16, 32768)
	size := uint32(len(buf))
	r, _, _ := procQueryFullProcessImageNameW.Call(h, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if r == 0 || size == 0 {
		return ""
	}
	full := syscall.UTF16ToString(buf[:size])
	return strings.ToLower(filepath.Base(full))
}

// ForegroundProcessName 返回当前前台窗口所属进程名（小写，含 .exe）。
func ForegroundProcessName() string {
	return processImageName(foregroundPID())
}

// IsProcessNameRunning 判断指定进程名是否仍有实例在运行。
func IsProcessNameRunning(name string) bool {
	name = normalizeProcessName(name)
	if name == "" {
		return false
	}
	snap, _, _ := procCreateToolhelp32Snapshot.Call(th32csSnapProcess, 0)
	if snap == 0 || snap == uintptr(syscall.InvalidHandle) {
		return false
	}
	defer procCloseHandle.Call(snap)

	var entry processEntry32W
	entry.Size = uint32(unsafe.Sizeof(entry))
	r, _, _ := procProcess32FirstW.Call(snap, uintptr(unsafe.Pointer(&entry)))
	if r == 0 {
		return false
	}
	for {
		exe := strings.ToLower(syscall.UTF16ToString(entry.ExeFile[:]))
		if exe == name {
			return true
		}
		r, _, _ = procProcess32NextW.Call(snap, uintptr(unsafe.Pointer(&entry)))
		if r == 0 {
			break
		}
	}
	return false
}

func windowTitle(hwnd uintptr) string {
	n, _, _ := procGetWindowTextLengthW.Call(hwnd)
	if n == 0 {
		return ""
	}
	buf := make([]uint16, n+1)
	_, _, _ = procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(n+1))
	return strings.TrimSpace(syscall.UTF16ToString(buf))
}

// ListWindowProcesses 枚举当前有可见顶层窗口的进程（按进程名去重）。
func ListWindowProcesses() []ProcessInfo {
	self := currentPID()
	type acc struct {
		name  string
		pid   uint32
		title string
	}
	byName := map[string]acc{}

	cb := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		vis, _, _ := procIsWindowVisible.Call(hwnd)
		if vis == 0 {
			return 1
		}
		owner, _, _ := procGetWindow.Call(hwnd, gwOwner)
		if owner != 0 {
			return 1
		}
		title := windowTitle(hwnd)
		if title == "" {
			return 1
		}
		var pid uint32
		_, _, _ = procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		if pid == 0 || pid == self {
			return 1
		}
		name := processImageName(pid)
		if name == "" {
			return 1
		}
		// 跳过系统壳层常见项（可选保留 explorer）
		if name == "applicationframehost.exe" || name == "textinputhost.exe" {
			return 1
		}
		if old, ok := byName[name]; !ok || (old.title == "" && title != "") {
			byName[name] = acc{name: name, pid: pid, title: title}
		}
		return 1
	})
	_, _, _ = procEnumWindows.Call(cb, 0)

	out := make([]ProcessInfo, 0, len(byName))
	for _, v := range byName {
		out = append(out, ProcessInfo{Name: v.name, PID: v.pid, Title: v.title})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func normalizeProcessName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	name = strings.Trim(name, `"'`)
	if name == "" {
		return ""
	}
	return filepath.Base(name)
}
