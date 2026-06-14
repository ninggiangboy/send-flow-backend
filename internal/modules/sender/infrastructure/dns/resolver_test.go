package dns

import (
	"testing"
	"time"
)

func TestNewResolverDefaults(t *testing.T) {
	r := NewResolver()
	if r.timeout != 10*time.Second {
		t.Errorf("expected timeout 10s, got %v", r.timeout)
	}
}

func TestNewResolverCustom(t *testing.T) {
	r := NewResolver(Options{
		Timeout:  5 * time.Second,
		CacheTTL: 1 * time.Minute,
	})
	if r.timeout != 5*time.Second {
		t.Errorf("expected timeout 5s, got %v", r.timeout)
	}
}

func TestCache(t *testing.T) {
	c := newCache(100 * time.Millisecond)
	c.set("key1", "value1")
	val, ok := c.get("key1")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if val.(string) != "value1" {
		t.Errorf("expected value1, got %v", val)
	}
}

func TestCacheExpiry(t *testing.T) {
	c := newCache(50 * time.Millisecond)
	c.set("key1", "value1")
	time.Sleep(100 * time.Millisecond)
	_, ok := c.get("key1")
	if ok {
		t.Error("expected cache miss after expiry")
	}
}

func TestCacheMiss(t *testing.T) {
	c := newCache(1 * time.Minute)
	_, ok := c.get("nonexistent")
	if ok {
		t.Error("expected cache miss for nonexistent key")
	}
}

func TestCacheConcurrentAccess(t *testing.T) {
	c := newCache(1 * time.Minute)
	done := make(chan struct{}, 2)
	go func() {
		c.set("key1", "value1")
		c.get("key1")
		done <- struct{}{}
	}()
	go func() {
		c.set("key2", "value2")
		c.get("key2")
		done <- struct{}{}
	}()
	<-done
	<-done
}

func TestCacheStore(t *testing.T) {
	r := NewResolver(Options{Timeout: 1 * time.Second})
	r.cache.set("txt:test.host", []string{"v=spf1 include:amazonses.com ~all"})

	val, ok := r.cache.get("txt:test.host")
	if !ok {
		t.Fatal("expected cache hit")
	}
	vals := val.([]string)
	if len(vals) != 1 || vals[0] != "v=spf1 include:amazonses.com ~all" {
		t.Errorf("unexpected cached value: %v", vals)
	}
}

func TestCNAMECacheStore(t *testing.T) {
	r := NewResolver(Options{Timeout: 1 * time.Second})
	r.cache.set("cname:s1._domainkey.example.com", "s1.dkim.amazonses.com")

	val, ok := r.cache.get("cname:s1._domainkey.example.com")
	if !ok {
		t.Fatal("expected cache hit")
	}
	target := val.(string)
	if target != "s1.dkim.amazonses.com" {
		t.Errorf("expected 's1.dkim.amazonses.com', got %q", target)
	}
}
