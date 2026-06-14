package dns

import (
	"context"
	"net"
	"sync"
	"time"
)

type cacheEntry struct {
	value     any
	expiresAt time.Time
}

type cache struct {
	mu    sync.RWMutex
	store map[string]cacheEntry
	ttl   time.Duration
}

func newCache(ttl time.Duration) *cache {
	return &cache{store: make(map[string]cacheEntry), ttl: ttl}
}

func (c *cache) get(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.store[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.value, true
}

func (c *cache) set(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store[key] = cacheEntry{value: value, expiresAt: time.Now().Add(c.ttl)}
}

type Options struct {
	Timeout    time.Duration
	CacheTTL   time.Duration
	CustomDial func(ctx context.Context, network, address string) (net.Conn, error)
}

type Resolver struct {
	resolver *net.Resolver
	cache    *cache
	timeout  time.Duration
}

func NewResolver(opts ...Options) *Resolver {
	timeout := 10 * time.Second
	cacheTTL := 5 * time.Minute
	if len(opts) > 0 {
		if opts[0].Timeout > 0 {
			timeout = opts[0].Timeout
		}
		if opts[0].CacheTTL > 0 {
			cacheTTL = opts[0].CacheTTL
		}
	}
	r := &Resolver{
		resolver: &net.Resolver{
			PreferGo: true,
		},
		cache:   newCache(cacheTTL),
		timeout: timeout,
	}
	if len(opts) > 0 && opts[0].CustomDial != nil {
		r.resolver.Dial = opts[0].CustomDial
	}
	return r
}

func (r *Resolver) LookupTXT(ctx context.Context, host string) ([]string, error) {
	if cached, ok := r.cache.get("txt:" + host); ok {
		return cached.([]string), nil
	}
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	values, err := r.resolver.LookupTXT(ctx, host)
	if err != nil {
		return nil, err
	}
	r.cache.set("txt:"+host, values)
	return values, nil
}

func (r *Resolver) LookupCNAME(ctx context.Context, host string) (string, error) {
	if cached, ok := r.cache.get("cname:" + host); ok {
		return cached.(string), nil
	}
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()
	target, err := r.resolver.LookupCNAME(ctx, host)
	if err != nil {
		return "", err
	}
	r.cache.set("cname:"+host, target)
	return target, nil
}
