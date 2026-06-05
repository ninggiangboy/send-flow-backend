package secrets

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

var ErrNotFound = errors.New("secret not found")

type Provider interface {
	Get(context.Context, string) (string, error)
}

type EnvProvider struct {
	prefix string
}

func NewEnvProvider(prefix string) EnvProvider {
	return EnvProvider{prefix: prefix}
}

func (p EnvProvider) Get(_ context.Context, name string) (string, error) {
	key := p.prefix + strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
	value := os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("%w: %s", ErrNotFound, name)
	}
	return value, nil
}

type StaticProvider map[string]string

func (p StaticProvider) Get(_ context.Context, name string) (string, error) {
	value, ok := p[name]
	if !ok || value == "" {
		return "", fmt.Errorf("%w: %s", ErrNotFound, name)
	}
	return value, nil
}
