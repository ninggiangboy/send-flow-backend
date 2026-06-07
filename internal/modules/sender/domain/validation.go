package domain

import (
	"fmt"
	"net/url"
	"strings"
)

func NormalizeDomain(raw string) (string, error) {
	d := strings.TrimSpace(raw)
	d = strings.ToLower(d)
	if d == "" {
		return "", fmt.Errorf("%w: empty domain", ErrDomainInvalid)
	}
	if strings.ContainsAny(d, " \t") {
		return "", fmt.Errorf("%w: domain contains whitespace", ErrDomainInvalid)
	}
	if strings.Contains(d, "://") || strings.HasPrefix(d, "//") {
		return "", fmt.Errorf("%w: domain must not include scheme", ErrDomainInvalid)
	}
	if strings.Contains(d, "/") {
		return "", fmt.Errorf("%w: domain must not include path", ErrDomainInvalid)
	}
	if strings.Contains(d, "?") {
		return "", fmt.Errorf("%w: domain must not include query string", ErrDomainInvalid)
	}
	if strings.Contains(d, ":") {
		return "", fmt.Errorf("%w: domain must not include port", ErrDomainInvalid)
	}
	if strings.HasPrefix(d, "*.") || strings.HasPrefix(d, "*") {
		return "", fmt.Errorf("%w: wildcard domains are not supported", ErrDomainInvalid)
	}
	if _, err := url.Parse(d); err == nil && strings.Contains(d, "%") {
		return "", fmt.Errorf("%w: domain must not be URL-encoded", ErrDomainInvalid)
	}
	if len(d) > 253 {
		return "", fmt.Errorf("%w: domain exceeds 253 characters", ErrDomainInvalid)
	}
	if !strings.Contains(d, ".") {
		return "", fmt.Errorf("%w: domain must contain at least one dot", ErrDomainInvalid)
	}
	labels := strings.Split(d, ".")
	for _, label := range labels {
		if len(label) > 63 {
			return "", fmt.Errorf("%w: domain label exceeds 63 characters", ErrDomainInvalid)
		}
		if label == "" {
			return "", fmt.Errorf("%w: domain contains empty label", ErrDomainInvalid)
		}
	}
	for _, r := range d {
		if r > 127 {
			return "", fmt.Errorf("%w: only ASCII domain names are supported", ErrDomainInvalid)
		}
	}
	return d, nil
}

func ValidateProvider(provider string) (Provider, error) {
	switch Provider(strings.TrimSpace(strings.ToLower(provider))) {
	case ProviderSES:
		return ProviderSES, nil
	case "":
		return ProviderSES, nil
	default:
		return "", fmt.Errorf("%w: unsupported provider %q", ErrProviderConfigInvalid, provider)
	}
}
