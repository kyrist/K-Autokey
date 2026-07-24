package main

// keys.go：界面键位标签 ↔ Windows 虚拟键码。
//
// 供 bootstrap（key_choices）与 LabelsToVKs（连发配置）使用。
// 旧配置中的 Shift/Ctrl/Alt 在 LabelsToVKs 中映射为左侧键。

// KeyChoices 键位显示名 → 虚拟键码。
var KeyChoices = map[string]uint16{
	"Esc": 0x1B,
	"F1": 0x70, "F2": 0x71, "F3": 0x72, "F4": 0x73, "F5": 0x74, "F6": 0x75,
	"F7": 0x76, "F8": 0x77, "F9": 0x78, "F10": 0x79, "F11": 0x7A, "F12": 0x7B,

	"`": 0xC0, // OEM_3  ` ~
	"1": 0x31, "2": 0x32, "3": 0x33, "4": 0x34, "5": 0x35,
	"6": 0x36, "7": 0x37, "8": 0x38, "9": 0x39, "0": 0x30,
	"-": 0xBD, // OEM_MINUS
	"=": 0xBB, // OEM_PLUS
	"Back": 0x08,

	"Tab": 0x09,
	"Q": 0x51, "W": 0x57, "E": 0x45, "R": 0x52, "T": 0x54,
	"Y": 0x59, "U": 0x55, "I": 0x49, "O": 0x4F, "P": 0x50,
	"[": 0xDB, // OEM_4
	"]": 0xDD, // OEM_6
	"\\": 0xDC, // OEM_5

	"Caps": 0x14,
	"A": 0x41, "S": 0x53, "D": 0x44, "F": 0x46, "G": 0x47,
	"H": 0x48, "J": 0x4A, "K": 0x4B, "L": 0x4C,
	";": 0xBA, // OEM_1
	"'": 0xDE, // OEM_7
	"Enter": 0x0D,

	"Shift": 0xA0, // 兼容旧配置 → LShift
	"LShift": 0xA0,
	"RShift": 0xA1,
	"Z": 0x5A, "X": 0x58, "C": 0x43, "V": 0x56, "B": 0x42,
	"N": 0x4E, "M": 0x4D,
	",": 0xBC, // OEM_COMMA
	".": 0xBE, // OEM_PERIOD
	"/": 0xBF, // OEM_2

	"Ctrl":  0xA2, // 兼容旧配置 → LCtrl
	"LCtrl": 0xA2,
	"RCtrl": 0xA3,
	"Alt":   0xA4, // 兼容旧配置 → LAlt
	"LAlt":  0xA4,
	"RAlt":  0xA5,
	"Space": 0x20,
}

// keyOrder 供前端 bootstrap 使用（与键盘布局一致的可选键集合）。
var keyOrder = []string{
	"Esc",
	"F1", "F2", "F3", "F4", "F5", "F6", "F7", "F8", "F9", "F10", "F11", "F12",
	"`", "1", "2", "3", "4", "5", "6", "7", "8", "9", "0", "-", "=", "Back",
	"Tab", "Q", "W", "E", "R", "T", "Y", "U", "I", "O", "P", "[", "]", "\\",
	"Caps", "A", "S", "D", "F", "G", "H", "J", "K", "L", ";", "'", "Enter",
	"Shift", "Z", "X", "C", "V", "B", "N", "M", ",", ".", "/", "LShift", "RShift",
	"Ctrl", "Alt", "LCtrl", "RCtrl", "LAlt", "RAlt", "Space",
}

func KeyLabels() []string {
	return append([]string(nil), keyOrder...)
}

func LabelsToVKs(labels []string) []uint16 {
	out := make([]uint16, 0, len(labels))
	seen := map[uint16]struct{}{}
	for _, label := range labels {
		// 旧配置兼容
		switch label {
		case "Shift":
			label = "LShift"
		case "Ctrl":
			label = "LCtrl"
		case "Alt":
			label = "LAlt"
		}
		vk, ok := KeyChoices[label]
		if !ok {
			continue
		}
		if _, exists := seen[vk]; exists {
			continue
		}
		seen[vk] = struct{}{}
		out = append(out, vk)
	}
	if len(out) == 0 {
		return []uint16{0x20}
	}
	return out
}
