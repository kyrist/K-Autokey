package main

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
)

// ahk_engine.go：连发核心用 AutoHotkey v2 子进程实现。
//
// Go (Wails) 负责 UI/配置/托盘/进程绑定检测；AHK 负责连发核心：
// InstallKeybdHook + GetKeyState("P") + SendEvent vkFFscXX。
// AHK 的 LL 钩子在 DNF 前台能正常工作，绕过 DNF 对 Go 进程的拦截。
//
// 逻辑对齐 DNFAutoFire 的 core/AutoFire.ahk + core/SendIP.ahk + core/KeyConvert.ahk。

//go:embed embedded/AutoHotkey64.exe
var ahkExeFS []byte

//go:embed ahk/burst.ahk
var ahkBurstScript string

const ahkScriptName = "burst.ahk"

var (
	ahkMu         sync.Mutex
	ahkCmd        *exec.Cmd
	ahkScriptPath string
	ahkExePath    string
	ahkRunning    atomic.Bool
)

// ahkPrepare 释放 AutoHotkey64.exe 到程序数据目录。
func ahkPrepare() error {
	dir, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	dir = filepath.Join(dir, "K-Autokey")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	ahkExePath = filepath.Join(dir, "AutoHotkey64.exe")
	if err := os.WriteFile(ahkExePath, ahkExeFS, 0o755); err != nil {
		return err
	}
	ahkScriptPath = filepath.Join(dir, ahkScriptName)
	return nil
}

// ahkStart 生成并启动连发脚本。
// keys: 连发键的 AHK 键名列表（如 ["a","s","d"]）；intervalMs: 全局连发间隔；
// suppress: 是否吞掉物理键（AHK $ 热键）。
func ahkStart(keys []string, intervalMs int, suppress bool) error {
	ahkMu.Lock()
	defer ahkMu.Unlock()
	if ahkRunning.Load() {
		return nil
	}
	if err := ahkPrepare(); err != nil {
		return err
	}
	script := ahkRenderScript(keys, intervalMs, suppress)
	if err := os.WriteFile(ahkScriptPath, []byte(script), 0o644); err != nil {
		return err
	}
	cmd := exec.Command(ahkExePath, "/ErrorStdOut", ahkScriptPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	// 捕获 stderr 以诊断脚本错误
	pipe, err := cmd.StderrPipe()
	if err == nil {
		go func() {
			buf := make([]byte, 4096)
			for {
				n, e := pipe.Read(buf)
				if n > 0 {
					debugLog("ahk stderr: %s", string(buf[:n]))
				}
				if e != nil {
					return
				}
			}
		}()
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	ahkCmd = cmd
	ahkRunning.Store(true)
	go func() {
		err := cmd.Wait()
		ahkRunning.Store(false)
		debugLog("ahk process exit err=%v", err)
	}()
	return nil
}

// ahkStop 终止连发子进程。
func ahkStop() {
	ahkMu.Lock()
	defer ahkMu.Unlock()
	if !ahkRunning.Load() {
		return
	}
	if ahkCmd != nil && ahkCmd.Process != nil {
		_ = ahkCmd.Process.Kill()
	}
	ahkRunning.Store(false)
}

// ahkRunning_ 返回 AHK 子进程是否在运行（供状态查询）。
func ahkRunning_() bool { return ahkRunning.Load() }

// ahkRenderScript 生成 AHK v2 连发脚本，逻辑对齐 DNFAutoFire。
func ahkRenderScript(keys []string, intervalMs int, suppress bool) string {
	if intervalMs < 1 {
		intervalMs = 1
	}
	var b strings.Builder
	b.WriteString(ahkBurstScript)
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("global g_interval := %d\n", intervalMs))
	if suppress {
		b.WriteString("global g_suppress := true\n")
	} else {
		b.WriteString("global g_suppress := false\n")
	}
	b.WriteString("global g_keys := [")
	for i, k := range keys {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(fmt.Sprintf("%q", k))
	}
	b.WriteString("]\n\n")
	b.WriteString("BurstStartup()\n")
	return b.String()
}

// labelsToAHKKeyNames 把 VK 列表转成 AHK 键名（用于 GetKeySC/SendEvent）。
func labelsToAHKKeyNames(vks []uint16) []string {
	var names []string
	for _, vk := range vks {
		names = append(names, vkToAHKName(vk))
	}
	return names
}

// vkToAHKName 把 VK 转成 AHK 键名（用于 GetKeySC/SendEvent）。
func vkToAHKName(vk uint16) string {
	if name, ok := vkToAHKNameMap[vk]; ok {
		return name
	}
	return "vk" + strconv.FormatUint(uint64(vk), 16)
}
