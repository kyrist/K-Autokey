#Requires AutoHotkey v2.0
#SingleInstance Force

; burst.ahk - K-Autokey 连发核心（对齐 DNFAutoFire）
; 由 Go 生成并启动：InstallKeybdHook + GetKeyState("P") + SendEvent vkFFscXX

; DNF 游戏窗口组（对齐 DNFAutoFire 的 GAME_WINDOW_TITLES）
GroupAdd("DNF", "地下城与勇士：创新世纪")
GroupAdd("DNF", "次元对决")

; 安装键盘钩子，维护物理键状态（GetKeyState "P" 依赖此钩子）
InstallKeybdHook()

; 全局暂停（由 Go 通过命名 Event 控制，可选）
global g_paused := false

; 由 Go 注入的变量：g_interval（连发间隔 ms）、g_keys（AHK 键名数组）
; global g_interval := 20
; global g_keys := ["a","s","d","f","q","w","e","r","g"]

BurstStartup() {
    global g_keys, g_interval
    timers := []
    for idx, keyName in g_keys {
        pressKey := Key2PressKey(keyName)
        keyCode := Key2NoVkSC(keyName)
        fn := AutoFireSingleKeyTick.Bind(pressKey, keyCode)
        timers.Push(fn)
        SetTimer(fn, g_interval)
    }
}

; 单键连发 tick（对齐 DNFAutoFire 的 AutoFireSingleKeyTick）
AutoFireSingleKeyTick(pressKey, keyCode) {
    global g_paused
    if g_paused {
        return
    }
    if !WinActive("ahk_group DNF") {
        return
    }
    static keyBusy := Map()
    if keyBusy.Has(pressKey) && keyBusy[pressKey] {
        return
    }
    keyBusy[pressKey] := true
    try {
        if GetKeyState(pressKey, "P") {
            SendIP(keyCode, 8)
        }
    } finally {
        keyBusy[pressKey] := false
    }
}

; 发送一次 down/up（对齐 DNFAutoFire 的 SendIP）
SendIP(keyCode, keyDelayMs := 8) {
    keyDelayMs := Round(keyDelayMs + 0)
    if (keyDelayMs < 0) {
        keyDelayMs := 0
    }
    Critical("On")
    try {
        SetKeyDelay(-1, -1)
        SendEvent("{Blind}{" keyCode " DownTemp}")
        DllCall("Sleep", "UInt", keyDelayMs)
        SendEvent("{Blind}{" keyCode " Up}")
        DllCall("Sleep", "UInt", 2)
    } finally {
        Critical("Off")
    }
}

; 按键转只有扫描码（对齐 DNFAutoFire 的 Key2NoVkSC）
Key2NoVkSC(key) {
    sc := GetKeySC(key)
    return Format("vkFFsc{1:02X}", sc)
}

; 按键转检测物理按下的键名（对齐 DNFAutoFire 的 Key2PressKey）
Key2PressKey(key) {
    sc := GetKeySC(key)
    newKey := Format("sc{1:02X}", sc)
    if InStr(key, "Num") {
        newKey := key
    }
    return newKey
}
