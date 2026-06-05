package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

type FileProvider struct {
	values map[string]string
}

func NewFileProvider(path string) (FileProvider, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return FileProvider{}, fmt.Errorf("read secret file: %w", err)
	}
	values := map[string]string{}
	if err := json.Unmarshal(data, &values); err != nil {
		return FileProvider{}, fmt.Errorf("parse secret file: %w", err)
	}
	return FileProvider{values: values}, nil
}

func (p FileProvider) Get(ctx context.Context, name string) (string, error) {
	return StaticProvider(p.values).Get(ctx, name)
}
