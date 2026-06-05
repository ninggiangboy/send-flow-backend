package featureflags

import (
	"context"
	"strings"
)

type EvaluationContext struct {
	TenantID string
	UserID   string
	Attrs    map[string]string
}

type Client interface {
	Enabled(context.Context, string, EvaluationContext) bool
}

type StaticClient struct {
	defaultValue bool
	flags        map[string]bool
	tenantFlags  map[string]map[string]bool
}

func NewStatic(defaultValue bool, flags map[string]bool) StaticClient {
	cp := make(map[string]bool, len(flags))
	for k, v := range flags {
		cp[strings.TrimSpace(k)] = v
	}
	return StaticClient{defaultValue: defaultValue, flags: cp, tenantFlags: map[string]map[string]bool{}}
}

func (c StaticClient) WithTenantFlag(tenantID, flag string, enabled bool) StaticClient {
	if c.tenantFlags == nil {
		c.tenantFlags = map[string]map[string]bool{}
	}
	if c.tenantFlags[tenantID] == nil {
		c.tenantFlags[tenantID] = map[string]bool{}
	}
	c.tenantFlags[tenantID][flag] = enabled
	return c
}

func (c StaticClient) Enabled(_ context.Context, flag string, ctx EvaluationContext) bool {
	flag = strings.TrimSpace(flag)
	if flag == "" {
		return false
	}
	if ctx.TenantID != "" {
		if tenantFlags, ok := c.tenantFlags[ctx.TenantID]; ok {
			if enabled, ok := tenantFlags[flag]; ok {
				return enabled
			}
		}
	}
	if enabled, ok := c.flags[flag]; ok {
		return enabled
	}
	return c.defaultValue
}
