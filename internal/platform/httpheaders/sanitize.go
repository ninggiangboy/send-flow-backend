package httpheaders

import "strings"

var sensitiveHeaders = map[string]bool{
	"authorization":       true,
	"cookie":              true,
	"set-cookie":          true,
	"x-api-key":           true,
	"x-auth-token":        true,
	"proxy-authorization": true,
}

func SanitizeHeaders(headers map[string][]string, allowed ...string) map[string]string {
	if len(allowed) > 0 {
		allowedSet := make(map[string]bool, len(allowed))
		for _, a := range allowed {
			allowedSet[strings.ToLower(a)] = true
		}
		result := make(map[string]string, len(allowed))
		for k, v := range headers {
			if allowedSet[strings.ToLower(k)] && len(v) > 0 {
				result[k] = v[0]
			}
		}
		return result
	}

	result := make(map[string]string, len(headers))
	for k, v := range headers {
		if sensitiveHeaders[strings.ToLower(k)] {
			continue
		}
		val := ""
		if len(v) > 0 {
			val = v[0]
		}
		if len(val) > 1000 {
			val = val[:1000]
		}
		result[k] = val
	}
	return result
}
