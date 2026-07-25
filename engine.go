package main

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// engine.go — 连发调度（对齐 DNFAutoFire.key_sender_thread）
//
//	armed   = 用户总开关（热键 / 按钮）
//	enabled = 当前是否真正连发（可被进程前台监视门控）
//
//	每个已绑键一个 fireLoop：
//	  for {
//	    if enabled && keyDown { send_key_once() }  // down + 1ms + up
//	    sleep(intervalMs)                          // 未按下也 sleep，避免空转占 CPU
//	  }

type RepeatSettings struct {
	KeyVKs     []uint16
	IntervalMs int
}

type Engine struct {
	mu       sync.Mutex
	settings RepeatSettings
	enabled  bool
	armed    bool
	stopCh   chan struct{}
	running  bool

	intervalMs  atomic.Int64
	enabledFlag atomic.Bool
	armedFlag   atomic.Bool

	workerStop map[uint16]chan struct{}
	onState    func(enabled bool)
}

func NewEngine(onState func(bool)) *Engine {
	e := &Engine{
		settings: RepeatSettings{
			KeyVKs:     []uint16{0x20},
			IntervalMs: 1,
		},
		workerStop: map[uint16]chan struct{}{},
		onState:    onState,
	}
	e.intervalMs.Store(1)
	return e
}

func (e *Engine) Configure(s RepeatSettings) {
	e.mu.Lock()
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
	keysChanged := !uint16SliceEqual(e.settings.KeyVKs, s.KeyVKs)
	e.settings = s
	e.intervalMs.Store(int64(s.IntervalMs))
	enabled := e.enabled
	keys := append([]uint16(nil), e.settings.KeyVKs...)
	e.mu.Unlock()

	if enabled && keysChanged {
		e.restartWorkers(keys)
	}
}

func uint16SliceEqual(a, b []uint16) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
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

func (e *Engine) IsInjecting() bool {
	// 连发开启且有任意物理键按住时，认为正在注入脉冲，
	// 让 HotkeyWatcher/ProcessFocusWatcher 在注入期间跳过误判。
	return e.enabledFlag.Load() && anyPhysicalKeyDown()
}

func (e *Engine) StartWatcher() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.running {
		return
	}
	e.stopCh = make(chan struct{})
	e.running = true
	enableHighResolutionTimer()
}

func (e *Engine) StopWatcher() {
	e.mu.Lock()
	if !e.running {
		e.mu.Unlock()
		return
	}
	close(e.stopCh)
	e.running = false
	e.enabled = false
	e.armed = false
	e.enabledFlag.Store(false)
	e.armedFlag.Store(false)
	e.mu.Unlock()

	e.stopAllWorkers()
	ahkStop()
	stopKeyHook()
	stopRawInput()
	disableHighResolutionTimer()
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

func (e *Engine) IsAutoPaused() bool       { return !e.IsArmed() }
func (e *Engine) ClearAutoPause()          { e.SetArmed(true) }
func (e *Engine) SetAutoPaused(v bool)     { e.SetArmed(!v) }
func (e *Engine) EmergencyStop()           { e.SetArmed(false) }

func (e *Engine) SyncSuppressMode() {
	e.mu.Lock()
	enabled := e.enabled
	keys := append([]uint16(nil), e.settings.KeyVKs...)
	e.mu.Unlock()
	applyBurstInputMode(enabled, keys)
	if enabled {
		e.restartWorkers(keys)
	}
}

func (e *Engine) emit(enabled bool) {
	if e.onState != nil {
		e.onState(enabled)
	}
}

func (e *Engine) stopAllWorkers() {
	e.mu.Lock()
	stops := e.workerStop
	e.workerStop = map[uint16]chan struct{}{}
	e.mu.Unlock()
	for _, ch := range stops {
		select {
		case <-ch:
		default:
			close(ch)
		}
	}
}

func (e *Engine) restartWorkers(keys []uint16) {
	e.stopAllWorkers()
	e.mu.Lock()
	for _, vk := range keys {
		stop := make(chan struct{})
		e.workerStop[vk] = stop
		go e.fireLoop(vk, stop)
	}
	e.mu.Unlock()
}

// fireLoop ≈ DNFAutoFire.key_sender_thread
//
// 直接 keyTap 发送脉冲（不使用队列）。松开按键时通过 notifyPhysicalChange
// 广播唤醒，立即停止发送；并在下降沿补发一次 KeyUp 兜底，防止键卡住。
func (e *Engine) fireLoop(vk uint16, stop <-chan struct{}) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	wasDown := false
	tick := 0
	for {
		select {
		case <-stop:
			return
		default:
		}

		down := e.enabledFlag.Load() && asyncKeyDown(vk)
		if down != wasDown {
			debugLog("fireLoop vk=0x%X down=%v", vk, down)
		}
		tick++
		if tick%200 == 0 {
			debugLog("fireLoop heartbeat vk=0x%X down=%v enabled=%v", vk, down, e.enabledFlag.Load())
		}
		if down {
			keyTap(vk)
		}
		// 检测按下→松开的下降沿：补发一次 KeyUp 兜底，确保该键不会卡在按下态。
		if wasDown && !down {
			keyUp(vk)
		}
		wasDown = down
		interval := time.Duration(e.intervalMs.Load()) * time.Millisecond
		if interval < time.Millisecond {
			interval = time.Millisecond
		}
		// 物理键松开时 notifyPhysicalChange 广播唤醒，立即停止脉冲；
		// 否则最多等 interval 后再轮询，未按下也 sleep，避免空转占 CPU。
		if !waitKeyOrStop(stop, interval) {
			return
		}
	}
}


