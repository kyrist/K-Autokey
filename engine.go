package main

import (
	"sync"
	"sync/atomic"
)

// engine.go — 连发总开关与生命周期管理。
//
// 连发核心由 AHK 子进程实现（ahk_engine.go + ahk/burst.ahk）：
// 物理键检测、脉冲发送、DNF 前台判断都在 AHK 侧完成。
//
// Engine 只负责：
//   - armed   = 用户总开关（热键 / 按钮）
//   - enabled = 当前是否真正连发（可被进程前台监视门控）
//   - 把启停与配置（按键、间隔）翻译成 AHK 子进程的启动 / 停止
//
// 不再维护任何 Go 侧的发送线程、物理键状态或吞键钩子。

type RepeatSettings struct {
	KeyVKs     []uint16
	IntervalMs int
}

type Engine struct {
	mu       sync.Mutex
	settings RepeatSettings
	enabled  bool
	armed    bool
	running  bool

	intervalMs  atomic.Int64
	enabledFlag atomic.Bool
	armedFlag   atomic.Bool

	onState func(enabled bool)
}

func NewEngine(onState func(bool)) *Engine {
	e := &Engine{
		settings: RepeatSettings{
			KeyVKs:     []uint16{0x20},
			IntervalMs: 1,
		},
		onState: onState,
	}
	e.intervalMs.Store(1)
	return e
}

func (e *Engine) Configure(s RepeatSettings) {
	if s.IntervalMs < 1 {
		s.IntervalMs = 1
	}
	if s.IntervalMs > 10000 {
		s.IntervalMs = 10000
	}
	if len(s.KeyVKs) > 0 {
		s.KeyVKs = uniqueU16(s.KeyVKs)
	} else {
		s.KeyVKs = append([]uint16(nil), e.settings.KeyVKs...)
	}
	e.mu.Lock()
	e.settings = s
	e.intervalMs.Store(int64(s.IntervalMs))
	enabled := e.enabled
	keys := append([]uint16(nil), s.KeyVKs...)
	interval := int(e.intervalMs.Load())
	e.mu.Unlock()

	// 已开启时改配置：重启 AHK 子进程以应用新键位 / 间隔。
	if enabled {
		names := labelsToAHKKeyNames(keys)
		if err := ahkStart(names, interval); err != nil {
			debugLog("Engine.Configure ahkStart error: %v", err)
		}
	}
}

func uniqueU16(in []uint16) []uint16 {
	seen := map[uint16]struct{}{}
	out := make([]uint16, 0, len(in))
	for _, v := range in {
		if v == 0 {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	if len(out) == 0 {
		return []uint16{0x20}
	}
	return out
}

func (e *Engine) Snapshot() RepeatSettings {
	e.mu.Lock()
	defer e.mu.Unlock()
	cp := e.settings
	cp.KeyVKs = append([]uint16(nil), e.settings.KeyVKs...)
	return cp
}

func (e *Engine) IsEnabled() bool { return e.enabledFlag.Load() }

func (e *Engine) StartWatcher() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.running = true
}

func (e *Engine) StopWatcher() {
	e.mu.Lock()
	if !e.running {
		e.mu.Unlock()
		return
	}
	e.running = false
	e.enabled = false
	e.armed = false
	e.enabledFlag.Store(false)
	e.armedFlag.Store(false)
	e.mu.Unlock()

	ahkStop()
	e.emit(false)
}

func (e *Engine) SetEnabled(on bool) {
	e.mu.Lock()
	if e.enabled == on {
		e.mu.Unlock()
		return
	}
	e.enabled = on
	e.enabledFlag.Store(on)
	keys := append([]uint16(nil), e.settings.KeyVKs...)
	interval := int(e.intervalMs.Load())
	e.mu.Unlock()
	debugLog("Engine.SetEnabled(%v) keys=%v interval=%d", on, keys, interval)

	if on {
		names := labelsToAHKKeyNames(keys)
		if err := ahkStart(names, interval); err != nil {
			debugLog("ahkStart error: %v", err)
		} else {
			debugLog("ahkStart ok keys=%v", names)
		}
	} else {
		ahkStop()
		debugLog("ahkStop done")
	}
	e.emit(on)
}

func (e *Engine) IsArmed() bool { return e.armedFlag.Load() }

func (e *Engine) SetArmed(on bool) {
	e.mu.Lock()
	e.armed = on
	e.armedFlag.Store(on)
	e.mu.Unlock()
	if !on {
		e.SetEnabled(false)
	}
}

func (e *Engine) IsAutoPaused() bool    { return !e.IsArmed() }
func (e *Engine) ClearAutoPause()        { e.SetArmed(true) }
func (e *Engine) SetAutoPaused(v bool)   { e.SetArmed(!v) }
func (e *Engine) EmergencyStop()         { e.SetArmed(false) }

func (e *Engine) emit(enabled bool) {
	if e.onState != nil {
		e.onState(enabled)
	}
}
