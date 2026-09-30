package platform

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Protect known executor credentials in exported events. Native history remains private.
func (x *Codex) redactor(a Agent) func(string) string {
	secrets := map[string]bool{}
	var collect func(any)
	collect = func(v any) {
		switch value := v.(type) {
		case string:
			if len(value) >= 12 {
				secrets[value] = true
			}
		case map[string]any:
			for _, child := range value {
				collect(child)
			}
		case []any:
			for _, child := range value {
				collect(child)
			}
		}
	}
	if raw, err := os.ReadFile(filepath.Join(x.authHome(), "auth.json")); err == nil {
		var value any
		if json.Unmarshal(raw, &value) == nil {
			collect(value)
		}
	}
	if env, err := executorEnv(a, ""); err == nil {
		for _, entry := range env {
			key, value, _ := strings.Cut(entry, "=")
			key = strings.ToUpper(key)
			if len(value) >= 8 && (strings.Contains(key, "TOKEN") || strings.Contains(key, "SECRET") || strings.Contains(key, "PASSWORD") || strings.Contains(key, "API_KEY") || strings.Contains(key, "CREDENTIAL")) {
				secrets[value] = true
			}
		}
	}
	values := make([]string, 0, len(secrets))
	for value := range secrets {
		values = append(values, value)
	}
	sort.Slice(values, func(i, j int) bool { return len(values[i]) > len(values[j]) })
	return func(text string) string {
		for _, secret := range values {
			text = strings.ReplaceAll(text, secret, "[redacted]")
		}
		return text
	}
}
func redactJSON(raw []byte, redact func(string) string) []byte {
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return raw
	}
	var walk func(any) any
	walk = func(v any) any {
		switch item := v.(type) {
		case string:
			return redact(item)
		case map[string]any:
			for k, v := range item {
				item[k] = walk(v)
			}
		case []any:
			for i, v := range item {
				item[i] = walk(v)
			}
		}
		return v
	}
	result, err := json.Marshal(walk(value))
	if err != nil {
		return raw
	}
	return result
}
