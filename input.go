package main

import (
	"time"
)

// input.go：输入发送与吞键路由。
//
// 发送：keybd_event(vk=0xFF + 扫描码)（对齐 DNFAutoFire 的 SendEvent vkFFscXX）。
//   - DNF 按扫描码响应触发连发，vk=0xFF 不影响聊天框打字；
//   - keybd_event 走 OS 输入队列，有背压，不会堆积阻塞回车。
//
// 吞键：WH_KEYBOARD_LL 低级钩子（对齐 AHK $ 热键），连发开启时吞掉已绑键的
// 物理事件，避免物理按下与脉冲叠加；引擎再发送脉冲。
//
// 物理按下检测：钩子激活时读 physDown（钩子维护），否则读 GetAsyncKeyState。

const keyTapHold = time.Millisecond // 对齐 DNF KEY_PRESS_TIME=1ms

var suppressPhys = true // true=AHK $ 吞键；false=AHK ~ 透传

func SetSuppressPhysical(on bool) { suppressPhys = on }

// InputStatus 供前端展示当前发送/吞键状态。
type InputStatus struct {
	Active  string `json:"active"`
	Message string `json:"message"`
}

func GetInputStatus() InputStatus {
	msg := "用户态 Event（keybd_event vk=0xFF + 扫描码）"
	if suppressPhys {
		if !keyHookReady() {
			msg += "；吞键钩子未就绪"
		} else if suppressOn.Load() {
			msg += "；已吞物理键（$）"
		}
	} else {
		msg += "；透传物理键（~）"
	}
	return InputStatus{Active: "sendinput", Message: msg}
}

// applyBurstInputMode 按启停登记连发键（供 Engine 启停共用）。
// Raw Input 始终运行，无需按启停安装；这里仅做物理键种子和吞键登记。
func applyBurstInputMode(enabled bool, keys []uint16) {
	debugLog("applyBurstInputMode(%v) keys=%v suppressPhys=%v", enabled, keys, suppressPhys)
	if !enabled {
		setBurstSuppress(false, nil)
		return
	}
	if suppressPhys {
		setBurstSuppress(true, keys)
		seedPhysicalKeys(keys)
		return
	}
	setBurstSuppress(false, nil)
}

// keyTap 发送一次 down/up 脉冲（vk=0xFF + 扫描码）。
func keyTap(vk uint16) {
	sendInputKeyTap(vk)
}

// keyUp 只发送一次 KeyUp，用于松开按键时兜底，防止连发键卡在按下态。
func keyUp(vk uint16) {
	sendOurKey(vk, true)
}

// asyncKeyDown 检测物理键是否按住。
// 用 Raw Input 维护的 physDown（内核级输入，DNF 前台也能收到）。
func asyncKeyDown(vk uint16) bool {
	return physicalKeyDown(vk)
}

// hotkeyKeyDown 热键专用按键检测：直接读 GetAsyncKeyState（热键不吞）。
func hotkeyKeyDown(vk uint16) bool {
	return sendInputAsyncKeyDown(vk)
}
