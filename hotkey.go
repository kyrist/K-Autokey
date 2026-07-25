package main

import (
	"strings"
)

// hotkey.go：组合热键字符串的解析、规范化与按下检测。
//
// 存储格式示例：f6、ctrl+shift+a、lalt+f8
// 检测为精确匹配：未声明的 Ctrl/Shift/Alt/Win 不得按下。

// ModifierReq 修饰键要求。
type ModifierReq int

const (
	ModNone ModifierReq = iota // 不需要（按下则不算命中）
	ModAny                     // 左右任一
	ModLeft
	ModRight
)

// ParsedHotkey 解析后的组合热键。
type ParsedHotkey struct {
	Ctrl  ModifierReq
	Shift ModifierReq
	Alt   ModifierReq
	Win   ModifierReq
	KeyVK uint16
	Key   string // 主键规范名，如 f6、a、space
}

func (h ParsedHotkey) Valid() bool {
	return h.KeyVK != 0 && h.Key != ""
}

// Canonical 返回存储格式，如 ctrl+shift+f6。
func (h ParsedHotkey) Canonical() string {
	if !h.Valid() {
		return ""
	}
	parts := make([]string, 0, 5)
	parts = append(parts, modToken("ctrl", h.Ctrl)...)
	parts = append(parts, modToken("shift", h.Shift)...)
	parts = append(parts, modToken("alt", h.Alt)...)
	parts = append(parts, modToken("win", h.Win)...)
	parts = append(parts, h.Key)
	return strings.Join(parts, "+")
}

func modToken(base string, req ModifierReq) []string {
	switch req {
	case ModAny:
		return []string{base}
	case ModLeft:
		return []string{"l" + base}
	case ModRight:
		return []string{"r" + base}
	default:
		return nil
	}
}

// Display 返回界面展示，如 Ctrl+Shift+F6。
func (h ParsedHotkey) Display() string {
	c := h.Canonical()
	if c == "" {
		return ""
	}
	parts := strings.Split(c, "+")
	for i, p := range parts {
		parts[i] = displayPart(p)
	}
	return strings.Join(parts, "+")
}

func displayPart(p string) string {
	switch p {
	case "ctrl", "lctrl", "rctrl":
		if p == "lctrl" {
			return "LCtrl"
		}
		if p == "rctrl" {
			return "RCtrl"
		}
		return "Ctrl"
	case "shift", "lshift", "rshift":
		if p == "lshift" {
			return "LShift"
		}
		if p == "rshift" {
			return "RShift"
		}
		return "Shift"
	case "alt", "lalt", "ralt":
		if p == "lalt" {
			return "LAlt"
		}
		if p == "ralt" {
			return "RAlt"
		}
		return "Alt"
	case "win", "lwin", "rwin":
		if p == "lwin" {
			return "LWin"
		}
		if p == "rwin" {
			return "RWin"
		}
		return "Win"
	case "esc":
		return "Esc"
	case "space":
		return "Space"
	case "back":
		return "Backspace"
	case "tab":
		return "Tab"
	case "enter":
		return "Enter"
	case "caps":
		return "Caps"
	default:
		if len(p) >= 2 && p[0] == 'f' {
			ok := true
			for i := 1; i < len(p); i++ {
				if p[i] < '0' || p[i] > '9' {
					ok = false
					break
				}
			}
			if ok {
				return strings.ToUpper(p)
			}
		}
		if len(p) == 1 && p[0] >= 'a' && p[0] <= 'z' {
			return strings.ToUpper(p)
		}
		return p
	}
}

// ParseHotkey 解析热键字符串；兼容旧版单键 f6。
func ParseHotkey(s string) (ParsedHotkey, bool) {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.ReplaceAll(s, " ", "")
	if s == "" {
		return ParsedHotkey{}, false
	}
	parts := strings.Split(s, "+")
	var h ParsedHotkey
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		switch part {
		case "ctrl", "control", "controlkey":
			h.Ctrl = ModAny
		case "lctrl", "lcontrol":
			h.Ctrl = ModLeft
		case "rctrl", "rcontrol":
			h.Ctrl = ModRight
		case "shift":
			h.Shift = ModAny
		case "lshift":
			h.Shift = ModLeft
		case "rshift":
			h.Shift = ModRight
		case "alt", "menu", "option":
			h.Alt = ModAny
		case "lalt", "lmenu":
			h.Alt = ModLeft
		case "ralt", "rmenu":
			h.Alt = ModRight
		case "win", "meta", "super", "cmd":
			h.Win = ModAny
		case "lwin", "lmeta":
			h.Win = ModLeft
		case "rwin", "rmeta":
			h.Win = ModRight
		default:
			vk := keyTokenToVK(part)
			if vk == 0 {
				return ParsedHotkey{}, false
			}
			if h.Key != "" {
				return ParsedHotkey{}, false
			}
			h.Key = normalizeKeyToken(part)
			h.KeyVK = vk
		}
	}
	if !h.Valid() {
		return ParsedHotkey{}, false
	}
	return h, true
}

func NormalizeHotkey(s string) string {
	h, ok := ParseHotkey(s)
	if !ok {
		return ""
	}
	return h.Canonical()
}

func FormatHotkeyDisplay(s string) string {
	h, ok := ParseHotkey(s)
	if !ok {
		return strings.ToUpper(s)
	}
	return h.Display()
}

func normalizeKeyToken(part string) string {
	switch part {
	case "escape":
		return "esc"
	case "return":
		return "enter"
	case "backspace":
		return "back"
	case "capslock":
		return "caps"
	default:
		return part
	}
}

func keyTokenToVK(name string) uint16 {
	name = normalizeKeyToken(name)
	switch name {
	case "f1":
		return 0x70
	case "f2":
		return 0x71
	case "f3":
		return 0x72
	case "f4":
		return 0x73
	case "f5":
		return 0x74
	case "f6":
		return 0x75
	case "f7":
		return 0x76
	case "f8":
		return 0x77
	case "f9":
		return 0x78
	case "f10":
		return 0x79
	case "f11":
		return 0x7A
	case "f12":
		return 0x7B
	case "esc":
		return 0x1B
	case "space":
		return 0x20
	case "tab":
		return 0x09
	case "enter":
		return 0x0D
	case "back":
		return 0x08
	case "caps":
		return 0x14
	case "`":
		return 0xC0
	case "-":
		return 0xBD
	case "=":
		return 0xBB
	case "[":
		return 0xDB
	case "]":
		return 0xDD
	case "\\":
		return 0xDC
	case ";":
		return 0xBA
	case "'":
		return 0xDE
	case ",":
		return 0xBC
	case ".":
		return 0xBE
	case "/":
		return 0xBF
	default:
		if len(name) == 1 {
			c := name[0]
			if c >= 'a' && c <= 'z' {
				return uint16(c - 32)
			}
			if c >= '0' && c <= '9' {
				return uint16(c)
			}
		}
		return 0
	}
}

func modDown(req ModifierReq, leftVK, rightVK uint16) bool {
	switch req {
	case ModNone:
		return !hotkeyKeyDown(leftVK) && !hotkeyKeyDown(rightVK)
	case ModAny:
		return hotkeyKeyDown(leftVK) || hotkeyKeyDown(rightVK)
	case ModLeft:
		return hotkeyKeyDown(leftVK)
	case ModRight:
		return hotkeyKeyDown(rightVK)
	default:
		return false
	}
}

// IsDown 精确匹配热键，读 GetAsyncKeyState（热键不吞，状态可靠）。
func (h ParsedHotkey) IsDown() bool {
	if !h.Valid() {
		return false
	}
	if !hotkeyKeyDown(h.KeyVK) {
		return false
	}
	if !modDown(h.Ctrl, 0xA2, 0xA3) {
		return false
	}
	if !modDown(h.Shift, 0xA0, 0xA1) {
		return false
	}
	if !modDown(h.Alt, 0xA4, 0xA5) {
		return false
	}
	if !modDown(h.Win, 0x5B, 0x5C) {
		return false
	}
	return true
}
