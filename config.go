package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// config.go：应用配置的加载、保存与归一化。
//
// 路径：%APPDATA%\K-Autokey\config.json
// App 持有一份内存中的 AppConfig，经 applyConfig 分发给各子系统。

// AppConfig 本地持久化配置（字段与前端 UIConfig / config.json 对齐）。
type AppConfig struct {
	KeyLabels       []string `json:"key_labels"`
	IntervalMs      int      `json:"interval_ms"`
	EnableHotkey    string   `json:"enable_hotkey"`
	EmergencyHotkey string   `json:"emergency_hotkey"`
	BoundProcess    string   `json:"bound_process"`
}

func DefaultConfig() AppConfig {
	return AppConfig{
		KeyLabels:       []string{"Space"},
		IntervalMs:      50,
		EnableHotkey:    "f6",
		EmergencyHotkey: "f8",
		BoundProcess:    "",
	}
}

func configPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "K-Autokey")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func LoadConfig() AppConfig {
	path, err := configPath()
	if err != nil {
		return DefaultConfig()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return DefaultConfig()
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return DefaultConfig()
	}
	return configFromMap(raw)
}

func SaveConfig(cfg AppConfig) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func (c AppConfig) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"key_labels":       c.KeyLabels,
		"interval_ms":      c.IntervalMs,
		"enable_hotkey":    c.EnableHotkey,
		"emergency_hotkey": c.EmergencyHotkey,
		"bound_process":    c.BoundProcess,
	}
}

func NormalizeUIConfig(cfg UIConfig) AppConfig {
	out := DefaultConfig()
	out.KeyLabels = normalizeStrings(cfg.KeyLabels, []string{"Space"})
	if cfg.IntervalMs >= 1 {
		out.IntervalMs = cfg.IntervalMs
	}
	if out.IntervalMs > 10000 {
		out.IntervalMs = 10000
	}
	if n := NormalizeHotkey(cfg.EnableHotkey); n != "" {
		out.EnableHotkey = n
	}
	if n := NormalizeHotkey(cfg.EmergencyHotkey); n != "" {
		out.EmergencyHotkey = n
	}
	if out.EnableHotkey == out.EmergencyHotkey {
		out.EmergencyHotkey = DefaultConfig().EmergencyHotkey
		if out.EnableHotkey == out.EmergencyHotkey {
			out.EnableHotkey = DefaultConfig().EnableHotkey
		}
	}
	out.BoundProcess = normalizeProcessName(cfg.BoundProcess)
	return out
}

func configFromMap(raw map[string]interface{}) AppConfig {
	cfg := DefaultConfig()
	if v, ok := raw["key_labels"]; ok {
		cfg.KeyLabels = normalizeStrings(toStringSlice(v), []string{"Space"})
	} else if v, ok := raw["key_label"].(string); ok && v != "" {
		cfg.KeyLabels = []string{v}
	}
	switch v := raw["interval_ms"].(type) {
	case float64:
		if int(v) >= 1 {
			cfg.IntervalMs = int(v)
		}
	case int:
		if v >= 1 {
			cfg.IntervalMs = v
		}
	}
	if cfg.IntervalMs > 10000 {
		cfg.IntervalMs = 10000
	}
	if cfg.IntervalMs < 1 {
		cfg.IntervalMs = 1
	}
	// 兼容旧 hold_hotkey 字段；组合键规范为 ctrl+shift+f6 形式
	if v, ok := raw["enable_hotkey"].(string); ok && v != "" {
		if n := NormalizeHotkey(v); n != "" {
			cfg.EnableHotkey = n
		}
	} else if v, ok := raw["hold_hotkey"].(string); ok && v != "" {
		if n := NormalizeHotkey(v); n != "" {
			cfg.EnableHotkey = n
		}
	}
	if v, ok := raw["emergency_hotkey"].(string); ok && v != "" {
		if n := NormalizeHotkey(v); n != "" {
			cfg.EmergencyHotkey = n
		}
	}
	if v, ok := raw["bound_process"].(string); ok {
		cfg.BoundProcess = normalizeProcessName(v)
	}
	return cfg
}

func toStringSlice(v interface{}) []string {
	arr, ok := v.([]interface{})
	if !ok {
		if s, ok := v.(string); ok && s != "" {
			return []string{s}
		}
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func normalizeStrings(in, fallback []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	if len(out) == 0 {
		return append([]string(nil), fallback...)
	}
	return out
}
