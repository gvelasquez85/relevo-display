//go:build windows

package windriver

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallerToolsDoNotFollowOverriddenSystemRoot(t *testing.T) {
	fake := filepath.Join(t.TempDir(), "fake-windows")
	t.Setenv("SystemRoot", fake)
	dir, e := trustedSystemDirectory()
	if e != nil {
		t.Fatal(e)
	}
	if !filepath.IsAbs(dir) || strings.HasPrefix(strings.ToLower(dir), strings.ToLower(fake)) {
		t.Fatalf("untrusted system directory %q", dir)
	}
}
