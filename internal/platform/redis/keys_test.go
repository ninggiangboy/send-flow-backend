package redis

import (
	"testing"
)

func TestKeyIdentityWorkspaceAccess(t *testing.T) {
	got := KeyIdentityWorkspaceAccess("ws1", "u1")
	want := "cache:identity:workspace-access:v1:ws1:u1"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestKeyIdentityWorkspaceSettings(t *testing.T) {
	got := KeyIdentityWorkspaceSettings("ws1")
	want := "cache:identity:workspace-settings:v1:ws1"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestKeyContentTemplatePreview(t *testing.T) {
	got := KeyContentTemplatePreview("ws1", "tpl1", "hash123")
	want := "cache:content:template-preview:v1:ws1:tpl1:hash123"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestKeySenderDomainAuth(t *testing.T) {
	got := KeySenderDomainAuth("ws1", "dom1")
	want := "cache:sender:domain-auth:v1:ws1:dom1"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestKeyAudienceSegmentMembership(t *testing.T) {
	got := KeyAudienceSegmentMembership("ws1", "seg1")
	want := "cache:audience:segment-membership:v1:ws1:seg1"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestKeyDeliveryIdempotency(t *testing.T) {
	got := KeyDeliveryIdempotency("ws1", "idem1")
	want := "idem:delivery:transactional:v1:ws1:idem1"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestKeyDeliveryQuota(t *testing.T) {
	got := KeyDeliveryQuota("ses", "ws1")
	want := "quota:delivery:provider:v1:ses:ws1"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestKeyLockMessage(t *testing.T) {
	got := KeyLockMessage("msg1")
	want := "lock:delivery:message:v1:msg1"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestKeyCacheLoadLock(t *testing.T) {
	got := KeyCacheLoadLock("cache:identity:workspace-settings:v1:ws1")
	want := "lock:cache:identity:workspace-settings:v1:ws1"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestPrefixIdentityWorkspaceAccess(t *testing.T) {
	got := PrefixIdentityWorkspaceAccess("ws1")
	want := "cache:identity:workspace-access:v1:ws1:"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestPrefixContentTemplatePreview(t *testing.T) {
	got := PrefixContentTemplatePreview("ws1", "tpl1")
	want := "cache:content:template-preview:v1:ws1:tpl1:"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestPrefixDeliveryIdempotency(t *testing.T) {
	got := PrefixDeliveryIdempotency("ws1")
	want := "idem:delivery:transactional:v1:ws1"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
