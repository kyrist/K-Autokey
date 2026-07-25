package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// debug.go：运行时诊断日志，写入 %APPDATA%\K-Autokey\debug.log。
// 用于定位连发/热键不生效等运行时问题。

var (
	debugMu  sync.Mutex
	debugOn  = true
	debugFile string
)

func debugLogInit() {
	dir, err := os.UserConfigDir()
	if err != nil {
		return
	}
	dir = filepath.Join(dir, "K-Autokey")
	_ = os.MkdirAll(dir, 0o755)
	debugFile = filepath.Join(dir, "debug.log")
	_ = os.WriteFile(debugFile, []byte(""), 0o644) // 每次启动清空
}

func debugLog(format string, args ...interface{}) {
	if !debugOn || debugFile == "" {
		return
	}
	debugMu.Lock()
	defer debugMu.Unlock()
	f, err := os.OpenFile(debugFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	ts := time.Now().Format("15:04:05.000")
	fmt.Fprintf(f, "%s %s\n", ts, fmt.Sprintf(format, args...))
}
