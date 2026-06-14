package buildinfo

import "testing"

func TestCurrentDefaults(t *testing.T) {
	info := Current()
	if info.Version == "" {
		t.Fatal("expected default version")
	}
	if info.GitSHA == "" {
		t.Fatal("expected default git sha")
	}
	if info.BuildTime == "" {
		t.Fatal("expected default build time")
	}
}
