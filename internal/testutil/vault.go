// Package testutil holds helpers shared by pholio's tests.
package testutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// RepoRoot returns the absolute path of the repository root.
func RepoRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..")
}

// CopyVault copies the fixture Vault testdata/vaults/<name>/ into a fresh
// t.TempDir() and returns its path, so tests can write without touching the
// checked-in fixture.
func CopyVault(t testing.TB, name string) string {
	t.Helper()
	src := filepath.Join(RepoRoot(), "testdata", "vaults", name)
	dst := t.TempDir()
	if err := os.CopyFS(dst, os.DirFS(src)); err != nil {
		t.Fatalf("copy fixture vault %q: %v", name, err)
	}
	return dst
}
