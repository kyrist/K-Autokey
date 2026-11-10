package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// debug.go：可选运行时诊断日志。
// 默认关闭；仅当环境变量 K_AUTOKEY_DEBUG=1 时写入 %APPDATA%\K-Autokey\debug.log。

var (
	debugMu   sync.Mutex
	debugOn   bool
	debugFile string
)

func debugLogInit() {
	if os.Getenv("K_AUTOKEY_DEBUG") != "1" {
		debugOn = false
		return
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return
	}
	dir = filepath.Join(dir, "K-Autokey")
	_ = os.MkdirAll(dir, 0o755)
	debugFile = filepath.Join(dir, "debug.log")
	_ = os.WriteFile(debugFile, []byte(""), 0o644) // 开启调试时每次启动清空
	debugOn = true
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
