#Requires AutoHotkey v2.0
#SingleInstance Force

; burst.ahk - K-Autokey 连发核心（对齐 DNFAutoFire）
; InstallKeybdHook + GetKeyState("P") + SendEvent vkFFscXX
; 可选吞物理键（$ 热键）：连发激活时吞掉物理按键事件，只让注入脉冲通过，
; 避免物理 auto-repeat 与注入脉冲叠加导致节奏不稳 / 卡键 / 回车阻塞。

; DNF 游戏窗口组（对齐 DNFAutoFire 的 GAME_WINDOW_TITLES）
GroupAdd("DNF", "地下城与勇士：创新世纪")
GroupAdd("DNF", "次元对决")

; 安装键盘钩子，维护物理键状态（GetKeyState "P" 依赖此钩子）
InstallKeybdHook()

; 全局暂停（由 Go 通过命名 Event 控制，可选）
global g_paused := false

; 由 Go 注入的变量：g_interval（连发间隔 ms）、g_keys（AHK 键名数组）、g_suppress（是否吞物理键）
; global g_interval := 20
; global g_keys := ["a","s","d","f","q","w","e","r","g"]
; global g_suppress := true

global g_suppressOn := false ; 当前吞键热键是否已开启
global g_holdStart := 0     ; 连发键首次检测到按下的时刻（A_TickCount），0=未按下

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
    ; 低频监视器：按 DNF 前台 + 暂停状态 + 按住时长动态开关吞键热键
    SetTimer(SuppressWatcher, 30)
}

; 吞键热键处理器：什么都不做，仅吞掉物理事件。
; 钩子仍会维护 GetKeyState("P") 的物理状态，连发 tick 仍能检测按下/松开。
SuppressDown(*) {
}
SuppressUp(*) {
}

; 按连发状态动态开关吞键热键。
; 只在「连发实际进行中」（DNF 前台 + 未暂停 + 连发键持续按住超过阈值）时才吞键，
; 避免在 DNF 聊天框打字或短暂按游戏键时被吞。
; 阈值：连发键持续按住 >= 150ms 才开启吞键，短暂敲击完全透传。
SuppressWatcher() {
    global g_keys, g_suppress, g_paused, g_suppressOn, g_holdStart
    if !g_suppress {
        if g_suppressOn {
            for keyName in g_keys {
                Hotkey("$" keyName, "Off")
                Hotkey("$" keyName " Up", "Off")
            }
            g_suppressOn := false
        }
        g_holdStart := 0
        return
    }
    anyDown := false
    if WinActive("ahk_group DNF") && !g_paused {
        for keyName in g_keys {
            if GetKeyState(Key2PressKey(keyName), "P") {
                anyDown := true
                break
            }
        }
    }
    if anyDown {
        if g_holdStart = 0 {
            g_holdStart := A_TickCount
        }
        ; 持续按住超过 150ms 才认为是连发意图，开启吞键
        want := (A_TickCount - g_holdStart) >= 150
    } else {
        g_holdStart := 0
        want := false
    }
    if (want = g_suppressOn) {
        return
    }
    g_suppressOn := want
    for keyName in g_keys {
        if want {
            Hotkey("$" keyName, SuppressDown, "On")
            Hotkey("$" keyName " Up", SuppressUp, "On")
        } else {
            Hotkey("$" keyName, "Off")
            Hotkey("$" keyName " Up", "Off")
        }
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
            SendIP(keyCode, 4)
        }
    } finally {
        keyBusy[pressKey] := false
    }
}

; 发送一次 down/up（对齐 DNFAutoFire 的 SendIP）
; keyDelayMs=4：按下保持 4ms；发送后 Sleep 1ms 间隔
SendIP(keyCode, keyDelayMs := 4) {
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
        DllCall("Sleep", "UInt", 1)
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
