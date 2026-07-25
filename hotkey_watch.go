package main

import (
	"sync"
	"time"
)

// HotkeyBindings 开启/紧急停止热键配置（与连发键位列表无关）。
type HotkeyBindings struct {
	Enable    string
	Emergency string
}

// BurstControl 热键监视器对连发引擎的最小依赖面。
type BurstControl interface {
	IsInjecting() bool
	EmergencyStop()
	ToggleEnabled()
}

// HotkeyWatcher 轮询全局热键边沿：按下开启热键则切换连发，按下紧急热键则停止。
//
// 通过 BurstControl 操作引擎，不读取 Engine 内部字段。
// 前端捕获热键期间应 SetListening(true)，避免误触发。
type HotkeyWatcher struct {
	burst BurstControl

	mu        sync.Mutex
	bindings  HotkeyBindings
	stopCh    chan struct{}
	running   bool
	listening bool // true 时不触发热键动作
	prevEn    bool
	prevEmg   bool
}

func NewHotkeyWatcher(burst BurstControl) *HotkeyWatcher {
	return &HotkeyWatcher{
		burst: burst,
		bindings: HotkeyBindings{
			Enable:    "f6",
			Emergency: "f8",
		},
	}
}

func (h *HotkeyWatcher) Configure(b HotkeyBindings) {
	h.mu.Lock()
	if b.Enable != "" {
		h.bindings.Enable = b.Enable
	}
	if b.Emergency != "" {
		h.bindings.Emergency = b.Emergency
	}
	h.mu.Unlock()
	h.SyncEdges()
}

func (h *HotkeyWatcher) Snapshot() HotkeyBindings {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.bindings
}

func (h *HotkeyWatcher) Start() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.running {
		return
	}
	h.stopCh = make(chan struct{})
	h.running = true
	go h.loop(h.stopCh)
}

func (h *HotkeyWatcher) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.running {
		return
	}
	close(h.stopCh)
	h.running = false
}

func (h *HotkeyWatcher) SetListening(on bool) {
	h.mu.Lock()
	h.listening = on
	h.mu.Unlock()
	if !on {
		h.SyncEdges()
	} else {
		h.mu.Lock()
		h.prevEn = false
		h.prevEmg = false
		h.mu.Unlock()
	}
}

// SyncEdges 将边沿基准对齐到当前按键状态（配置热键后调用，防止误触发）。
func (h *HotkeyWatcher) SyncEdges() {
	b := h.Snapshot()
	enHK, enOK := ParseHotkey(b.Enable)
	emgHK, emgOK := ParseHotkey(b.Emergency)
	en := enOK && enHK.IsDown()
	emg := emgOK && emgHK.IsDown()
	h.mu.Lock()
	h.prevEn = en
	h.prevEmg = emg
	h.mu.Unlock()
}

func (h *HotkeyWatcher) loop(stopCh <-chan struct{}) {
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			h.tick()
		}
	}
}

func (h *HotkeyWatcher) tick() {
	if h.burst.IsInjecting() {
		return
	}
	h.mu.Lock()
	listening := h.listening
	h.mu.Unlock()
	if listening {
		return
	}

	b := h.Snapshot()
	enHK, enOK := ParseHotkey(b.Enable)
	emgHK, emgOK := ParseHotkey(b.Emergency)

	en := enOK && enHK.IsDown()
	emg := emgOK && emgHK.IsDown()

	debugLog("hotkey tick: enable=%s ok=%v down=%v | emergency=%s ok=%v down=%v | prevEn=%v prevEmg=%v",
		b.Enable, enOK, en, b.Emergency, emgOK, emg, h.prevEn, h.prevEmg)

	h.mu.Lock()
	prevEn, prevEmg := h.prevEn, h.prevEmg
	h.prevEn, h.prevEmg = en, emg
	h.mu.Unlock()

	if emg && !prevEmg {
		debugLog("hotkey: emergency triggered")
		h.burst.EmergencyStop()
		return
	}
	if en && !prevEn {
		debugLog("hotkey: enable toggled")
		h.burst.ToggleEnabled()
	}
}
