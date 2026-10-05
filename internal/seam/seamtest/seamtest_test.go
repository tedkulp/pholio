package seamtest_test

import (
	"errors"
	"io/fs"
	"slices"
	"testing"

	"github.com/tedkulp/pholio/internal/seam/seamtest"
)

func names(t *testing.T, m *seamtest.MemFS, dir string) []string {
	t.Helper()
	entries, err := m.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}

func TestMemFSReadWrite(t *testing.T) {
	m := seamtest.NewMemFS(map[string]string{"/v/a.md": "a"})

	if _, err := m.ReadFile("/v/missing.md"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing file err = %v, want fs.ErrNotExist", err)
	}
	if err := m.WriteFile("/v/sub/b.md", []byte("b")); err != nil {
		t.Fatal(err)
	}
	got, err := m.ReadFile("/v/sub/b.md")
	if err != nil || string(got) != "b" {
		t.Fatalf("ReadFile = %q, %v", got, err)
	}
	if got := names(t, m, "/v"); !slices.Equal(got, []string{"a.md", "sub"}) {
		t.Fatalf("ReadDir /v = %v", got)
	}
	if info, err := m.Stat("/v/sub"); err != nil || !info.IsDir() {
		t.Fatalf("Stat /v/sub = %v, %v; want a directory", info, err)
	}
}

func TestMemFSRenameMovesFolderContents(t *testing.T) {
	m := seamtest.NewMemFS(map[string]string{"/v/old/a.md": "a", "/v/old/deep/b.md": "b"})

	if err := m.Rename("/v/old", "/v/new"); err != nil {
		t.Fatal(err)
	}
	if got, err := m.ReadFile("/v/new/deep/b.md"); err != nil || string(got) != "b" {
		t.Fatalf("moved file = %q, %v", got, err)
	}
	if _, err := m.Stat("/v/old"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("old folder still exists: %v", err)
	}
}

func TestTrashRemovesFromMemFS(t *testing.T) {
	m := seamtest.NewMemFS(map[string]string{"/v/a.md": "a"})
	tr := &seamtest.Trash{FS: m}

	if err := tr.Trash("/v/a.md"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Stat("/v/a.md"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("trashed file still exists: %v", err)
	}
	if got := tr.Trashed(); !slices.Equal(got, []string{"/v/a.md"}) {
		t.Fatalf("Trashed = %v", got)
	}
}
