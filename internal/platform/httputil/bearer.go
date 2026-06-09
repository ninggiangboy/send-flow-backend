package httputil

import "strings"

func ExtractBearerToken(authHeader string) (string, bool) {
	h := strings.TrimSpace(authHeader)
	if !strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return "", false
	}
	token := strings.TrimSpace(h[len("Bearer "):])
	if token == "" {
		return "", false
	}
	return token, true
}