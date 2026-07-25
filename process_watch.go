package main

import (
	"sync"
	"time"
)

// FocusBurstControl 进程焦点监视器对连发的依赖。
// armed = 热键/按钮总开关；enabled = 当前是否真正在连发。
type FocusBurstControl interface {
	IsInjecting() bool
	IsEnabled() bool
	IsArmed() bool
	SetEnabled(on bool)
}

// ProcessFocusWatcher：仅在「总开关 armed」打开时，按前台进程自动启停连发。
// 热键关闭（disarm）后绝不会自动再开。
type ProcessFocusWatcher struct {
	burst FocusBurstControl

	mu      sync.Mutex
	bound   string
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
	p.bound = name
	p.lastFG = ""
	p.mu.Unlock()
	p.syncNow()
}

func (p *ProcessFocusWatcher) Bound() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.bound
}

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

	// 总开关关闭：强制停连发，不改 armed
	if !p.burst.IsArmed() {
		if p.burst.IsEnabled() {
			p.burst.SetEnabled(false)
		}
		return
	}

	running := IsProcessNameRunning(name)
	fg := ForegroundProcessName()
	p.mu.Lock()
	p.lastFG = fg
	p.mu.Unlock()

	want := running && fg != "" && fg == name
	if want != p.burst.IsEnabled() {
		p.burst.SetEnabled(want)
	}
}
