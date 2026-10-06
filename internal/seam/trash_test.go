package seam_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tedkulp/pholio/internal/seam"
	"github.com/tedkulp/pholio/internal/seam/seamtest"
)

// trashIn is an XDG trash under a temp dir, never the user's own.
func trashIn(t *testing.T) (seam.SystemTrash, string, string) {
	t.Helper()
	tmp := t.TempDir()
	vault := filepath.Join(tmp, "my vault")
	if err := os.MkdirAll(filepath.Join(vault, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	trash := seam.SystemTrash{
		Dir:   filepath.Join(tmp, "data", "Trash"),
		Clock: seamtest.NewClock(time.Date(2026, 10, 5, 9, 30, 15, 0, time.Local)),
	}
	return trash, vault, trash.Dir
}

func write(t *testing.T, p, s string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSystemTrashMovesTheFileAndRecordsWhereItCameFrom(t *testing.T) {
	trash, vault, dir := trashIn(t)
	note := filepath.Join(vault, "a note.md")
	write(t, note, "hello")

	if err := trash.Trash(note); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(note); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the Note is still there: %v", err)
	}
	if got := read(t, filepath.Join(dir, "files", "a note.md")); got != "hello" {
		t.Errorf("trashed contents = %q", got)
	}
	want := "[Trash Info]\nPath=" + escapedPath(filepath.ToSlash(note)) + "\nDeletionDate=2026-10-05T09:30:15\n"
	if got := read(t, filepath.Join(dir, "info", "a note.md.trashinfo")); got != want {
		t.Errorf("trashinfo =\n%s\nwant\n%s", got, want)
	}
}

func TestSystemTrashKeepsEarlierItemsWithTheSameName(t *testing.T) {
	trash, vault, dir := trashIn(t)
	for i, s := range []string{"first", "second"} {
		p := filepath.Join(vault, "x.md")
		if i == 1 {
			p = filepath.Join(vault, "sub", "x.md")
		}
		write(t, p, s)
		if err := trash.Trash(p); err != nil {
			t.Fatal(err)
		}
	}
	if got := read(t, filepath.Join(dir, "files", "x.md")); got != "first" {
		t.Errorf("files/x.md = %q, want first", got)
	}
	if got := read(t, filepath.Join(dir, "files", "x.2.md")); got != "second" {
		t.Errorf("files/x.2.md = %q, want second", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "info", "x.2.md.trashinfo")); err != nil {
		t.Error(err)
	}
}

func TestSystemTrashTakesWholeFolders(t *testing.T) {
	trash, vault, dir := trashIn(t)
	write(t, filepath.Join(vault, "sub", "y.md"), "y")
	if err := trash.Trash(filepath.Join(vault, "sub")); err != nil {
		t.Fatal(err)
	}
	if got := read(t, filepath.Join(dir, "files", "sub", "y.md")); got != "y" {
		t.Errorf("files/sub/y.md = %q", got)
	}
}

func TestSystemTrashRefusesMissingFiles(t *testing.T) {
	trash, vault, dir := trashIn(t)
	if err := trash.Trash(filepath.Join(vault, "nope.md")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("err = %v, want not exist", err)
	}
	if entries, _ := os.ReadDir(filepath.Join(dir, "info")); len(entries) != 0 {
		t.Errorf("left %d trashinfo files behind", len(entries))
	}
}

func TestNewSystemTrashUsesXDGDataHome(t *testing.T) {
	env := map[string]string{"XDG_DATA_HOME": "/data"}
	got := seam.NewSystemTrash(func(k string) string { return env[k] }, "/home/u", nil)
	if got.Dir != filepath.Join("/data", "Trash") && got.Dir != filepath.Join("/home/u", ".Trash") {
		t.Errorf("Dir = %q", got.Dir)
	}
	env = map[string]string{}
	got = seam.NewSystemTrash(func(k string) string { return env[k] }, "/home/u", nil)
	if got.Dir != filepath.Join("/home/u", ".local", "share", "Trash") && got.Dir != filepath.Join("/home/u", ".Trash") {
		t.Errorf("default Dir = %q", got.Dir)
	}
}

// escapedPath is p as a trashinfo Path: URL-escaped, slashes kept.
func escapedPath(p string) string {
	out := ""
	for _, r := range []byte(p) {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '/', r == '-', r == '_', r == '.', r == '~':
			out += string(r)
		default:
			out += "%" + string("0123456789ABCDEF"[r>>4]) + string("0123456789ABCDEF"[r&15])
		}
	}
	return out
}
