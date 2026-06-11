package api

func parseLimitParam(s string, defaultVal, maxVal int) int {
	if s == "" {
		return defaultVal
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return defaultVal
		}
		n = n*10 + int(c-'0')
	}
	if n <= 0 {
		return defaultVal
	}
	if n > maxVal {
		return maxVal
	}
	return n
}
