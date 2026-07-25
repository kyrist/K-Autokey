package main

import (
	"sync"
)

// physical_keys.go：低级钩子维护的「物理键是否按住」状态。
//
// 钩子收到绑键 KeyDown 时置位、KeyUp 时清位。LL 钩子吞键后系统不再产生
// auto-repeat，故只靠 KeyDown/KeyUp 边沿维护，不需要心跳看门狗。

var (
	physMu   sync.RWMutex
	physDown = map[uint16]bool{}
)

func setPhysicalKey(vk uint16, down bool) {
	physMu.Lock()
	prev := physDown[vk]
	changed := prev != down
	if down {
		physDown[vk] = true
	} else {
		delete(physDown, vk)
	}
	physMu.Unlock()
	if changed {
		notifyPhysicalChange()
	}
}

func clearPhysicalKeys() {
	physMu.Lock()
	physDown = map[uint16]bool{}
	physMu.Unlock()
	notifyPhysicalChange()
}

// physicalKeyDown 返回该键是否按住（由钩子 KeyDown/KeyUp 边沿维护）。
func physicalKeyDown(vk uint16) bool {
	physMu.RLock()
	down := physDown[vk]
	physMu.RUnlock()
	return down
}

// anyPhysicalKeyDown 报告是否有任意物理键处于按住状态。
// 供 Engine.IsInjecting 判断连发是否正在注入脉冲。
func anyPhysicalKeyDown() bool {
	physMu.RLock()
	any := len(physDown) > 0
	physMu.RUnlock()
	return any
}

func seedPhysicalKeys(vks []uint16) {
	for _, vk := range vks {
		if sendInputAsyncKeyDown(vk) {
			// 仅登记物理按下态，不向 OS 注入任何合成事件。
			setPhysicalKey(vk, true)
		}
	}
}
