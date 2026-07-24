package main

import (
	"sync"
	"sync/atomic"
	"time"
)

// RepeatSettings 仅描述连发本身（键位 + 间隔），与热键、进程绑定无关。
type RepeatSettings struct {
	KeyVKs     []uint16
	IntervalMs int
}

// Engine 连发核心：开启后轮询已绑键，对当前按住的键按间隔 SendInput。
//
// 不感知全局热键与进程绑定；由 HotkeyWatcher / ProcessFocusWatcher / App 调用启停。
type Engine struct {
	mu         sync.Mutex
	settings   RepeatSettings
	enabled    bool
	autoPaused bool // 紧急停止/手动关闭后，禁止进程监视器立刻再自动打开
	stopCh     chan struct{}
	running    bool
	lastFire   map[uint16]time.Time

	injecting atomic.Bool
	onState   func(enabled bool) // 状态变更回调（由 App 接到前端事件与托盘）
}

func NewEngine(onState func(bool)) *Engine {
	return &Engine{
		settings: RepeatSettings{
			KeyVKs:     []uint16{0x20},
			IntervalMs: 50,
		},
		lastFire: map[uint16]time.Time{},
		onState:  onState,
	}
}

func (e *Engine) Configure(s RepeatSettings) {
	e.mu.Lock()
	defer e.mu.Unlock()
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
	e.settings = s
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

func (e *Engine) IsEnabled() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.enabled
}

// IsInjecting 是否正在向系统注入按键（供热键/进程监视器避让）。
func (e *Engine) IsInjecting() bool {
	return e.injecting.Load()
}

// StartWatcher 启动后台轮询（应用启动时调用一次）。
func (e *Engine) StartWatcher() {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return
	}
	e.stopCh = make(chan struct{})
	e.running = true
	stopCh := e.stopCh
	e.mu.Unlock()
	go e.loop(stopCh)
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
	e.mu.Unlock()
	e.emit(false)
}

// SetEnabled 开启/关闭连发功能。
func (e *Engine) SetEnabled(on bool) {
	e.mu.Lock()
	if e.enabled == on {
		e.mu.Unlock()
		return
	}
	e.enabled = on
	if !on {
		e.lastFire = map[uint16]time.Time{}
	}
	e.mu.Unlock()
	e.emit(on)
}

func (e *Engine) IsAutoPaused() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.autoPaused
}

func (e *Engine) ClearAutoPause() {
	e.SetAutoPaused(false)
}

func (e *Engine) SetAutoPaused(v bool) {
	e.mu.Lock()
	e.autoPaused = v
	e.mu.Unlock()
}

func (e *Engine) ToggleEnabled() {
	if e.IsEnabled() {
		e.SetAutoPaused(true)
		e.SetEnabled(false)
		return
	}
	e.SetAutoPaused(false)
	e.SetEnabled(true)
}

func (e *Engine) EmergencyStop() {
	e.SetAutoPaused(true)
	e.SetEnabled(false)
}

func (e *Engine) emit(enabled bool) {
	if e.onState != nil {
		e.onState(enabled)
	}
}

func (e *Engine) loop(stopCh <-chan struct{}) {
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			e.tick()
		}
	}
}

func (e *Engine) tick() {
	if e.injecting.Load() {
		return
	}
	s := e.Snapshot()
	e.mu.Lock()
	enabled := e.enabled
	e.mu.Unlock()
	if !enabled {
		return
	}

	now := time.Now()
	interval := time.Duration(s.IntervalMs) * time.Millisecond

	for _, vk := range s.KeyVKs {
		down := asyncKeyDown(vk)
		if !down {
			e.mu.Lock()
			delete(e.lastFire, vk)
			e.mu.Unlock()
			continue
		}

		e.mu.Lock()
		last, ok := e.lastFire[vk]
		due := !ok || now.Sub(last) >= interval
		if due {
			e.lastFire[vk] = now
		}
		e.mu.Unlock()

		if !due {
			continue
		}

		e.injecting.Store(true)
		keyTap(vk)
		e.injecting.Store(false)
	}
}
