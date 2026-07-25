package main

import (
	"sync"
	"syscall"
	"time"
)

// timing_windows.go：高精度等待。
// Go 1.23+ 的 time.Sleep / Timer 已使用高分辨 WaitableTimer（约 0.5ms），
// 再配合 timeBeginPeriod(1) 即可；避免自管全局 timer 句柄在多键下互抢。

var (
	winmm               = syscall.NewLazyDLL("winmm.dll")
	procTimeBeginPeriod = winmm.NewProc("timeBeginPeriod")
	procTimeEndPeriod   = winmm.NewProc("timeEndPeriod")
	timingPeriodOn      bool
)

func enableHighResolutionTimer() {
	if timingPeriodOn {
		return
	}
	r, _, _ := procTimeBeginPeriod.Call(1)
	if r == 0 {
		timingPeriodOn = true
	}
}

func disableHighResolutionTimer() {
	if !timingPeriodOn {
		return
	}
	_, _, _ = procTimeEndPeriod.Call(1)
	timingPeriodOn = false
}

func sleepPrecise(d time.Duration) {
	if d <= 0 {
		return
	}
	time.Sleep(d)
}
// waitKeyOrStop 空闲等待：物理键变化时广播唤醒，或短超时轮询。
func waitKeyOrStop(stop <-chan struct{}, poll time.Duration) bool {
	if poll <= 0 {
		poll = time.Millisecond
	}
	ch := currentPhysWake()
	t := time.NewTimer(poll)
	defer t.Stop()
	select {
	case <-stop:
		return false
	case <-ch:
		return true
	case <-t.C:
		return true
	}
}

var (
	physWakeMu sync.Mutex
	physWakeCh chan struct{}
)

func ensurePhysWake() {
	physWakeMu.Lock()
	if physWakeCh == nil {
		physWakeCh = make(chan struct{})
	}
	physWakeMu.Unlock()
}

func currentPhysWake() <-chan struct{} {
	ensurePhysWake()
	physWakeMu.Lock()
	ch := physWakeCh
	physWakeMu.Unlock()
	return ch
}

// notifyPhysicalChange 广播唤醒所有 fireLoop（close + 换新 channel）。
func notifyPhysicalChange() {
	ensurePhysWake()
	physWakeMu.Lock()
	close(physWakeCh)
	physWakeCh = make(chan struct{})
	physWakeMu.Unlock()
}

