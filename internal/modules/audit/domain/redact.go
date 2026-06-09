package domain

import (
	"strings"
)

var redactedKeys = map[string]struct{}{
	"password":      {},
	"token":         {},
	"secret":        {},
	"authorization": {},
	"cookie":        {},
	"api_key":       {},
	"access_token":  {},
	"refresh_token": {},
	"credential":    {},
	"private_key":   {},
}

const redactionPlaceholder = "***REDACTED***"
const maxStringLength = 500
const maxArrayItems = 50
const maxMapKeys = 50

func RedactPayload(payload map[string]any) map[string]any {
	if payload == nil {
		return nil
	}
	return redactValue(payload).(map[string]any)
}

func redactValue(val any) any {
	switch v := val.(type) {
	case string:
		if len(v) > maxStringLength {
			return v[:maxStringLength] + "..."
		}
		return v
	case map[string]any:
		result := make(map[string]any, len(v))
		keysProcessed := 0
		for key, innerVal := range v {
			if keysProcessed >= maxMapKeys {
				result["...truncated"] = true
				break
			}
			keyLower := strings.ToLower(key)
			if _, ok := redactedKeys[keyLower]; ok {
				result[key] = redactionPlaceholder
			} else {
				result[key] = redactValue(innerVal)
			}
			keysProcessed++
		}
		return result
	case []any:
		if len(v) > maxArrayItems {
			v = v[:maxArrayItems]
		}
		redacted := make([]any, len(v))
		for i, item := range v {
			redacted[i] = redactValue(item)
		}
		return redacted
	default:
		return v
	}
}
