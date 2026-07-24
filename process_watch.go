package main

import (
	"sync"
	"time"
)

// FocusBurstControl 进程焦点监视器对连发引擎的最小依赖面。
type FocusBurstControl interface {
	IsInjecting() bool
	IsEnabled() bool
	IsAutoPaused() bool
	ClearAutoPause()
	SetEnabled(on bool)
}

// ProcessFocusWatcher 在绑定进程位于前台时自动开启连发，切走或进程退出时关闭。
//
// 按进程映像名匹配（如 game.exe），不绑定 PID。通过 FocusBurstControl 操作引擎。
type ProcessFocusWatcher struct {
	burst FocusBurstControl

	mu      sync.Mutex
	bound   string // 小写进程名；空表示未绑定（本监视器空转）
	stopCh  chan struct{}
	running bool
	lastFG  string
}

func NewProcessFocusWatcher(burst FocusBurstControl) *ProcessFocusWatcher {
	return &ProcessFocusWatcher{burst: burst}
}

func (p *ProcessFocusWatcher) Start() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.running {
		return
	}
	p.stopCh = make(chan struct{})
	p.running = true
	go p.loop(p.stopCh)
}

func (p *ProcessFocusWatcher) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.running {
		return
	}
	close(p.stopCh)
	p.running = false
}

func (p *ProcessFocusWatcher) SetBound(name string) {
	name = normalizeProcessName(name)
	p.mu.Lock()
	prev := p.bound
	p.bound = name
	p.lastFG = ""
	p.mu.Unlock()
	if name == "" && prev != "" {
		p.burst.ClearAutoPause()
		return
	}
	if name != "" {
		p.burst.ClearAutoPause()
		p.syncNow()
	}
}

func (p *ProcessFocusWatcher) Bound() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.bound
}

// IsTargetForeground 绑定进程是否正在运行且位于前台。
func (p *ProcessFocusWatcher) IsTargetForeground() bool {
	p.mu.Lock()
	name := p.bound
	p.mu.Unlock()
	if name == "" {
		return false
	}
	return IsProcessNameRunning(name) && ForegroundProcessName() == name
}

func (p *ProcessFocusWatcher) loop(stopCh <-chan struct{}) {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			p.tick()
		}
	}
}

func (p *ProcessFocusWatcher) syncNow() {
	p.tick()
}

func (p *ProcessFocusWatcher) tick() {
	p.mu.Lock()
	name := p.bound
	p.mu.Unlock()
	if name == "" {
		return
	}
	if p.burst.IsInjecting() {
		return
	}

	if !IsProcessNameRunning(name) {
		p.mu.Lock()
		p.lastFG = ""
		p.mu.Unlock()
		p.burst.ClearAutoPause()
		if p.burst.IsEnabled() {
			p.burst.SetEnabled(false)
		}
		return
	}

	fg := ForegroundProcessName()
	p.mu.Lock()
	p.lastFG = fg
	p.mu.Unlock()

	match := fg != "" && fg == name
	if !match {
		p.burst.ClearAutoPause()
		if p.burst.IsEnabled() {
			p.burst.SetEnabled(false)
		}
		return
	}

	if p.burst.IsAutoPaused() {
		return
	}
	if !p.burst.IsEnabled() {
		p.burst.SetEnabled(true)
	}
}
