package app_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/tedkulp/pholio/internal/app"
	"github.com/tedkulp/pholio/internal/config"
	"github.com/tedkulp/pholio/internal/seam"
	"github.com/tedkulp/pholio/internal/seam/seamtest"
	"github.com/tedkulp/pholio/internal/watch"
)

// watched opens /vault/a.md with a Vault watcher over a fake Watcher.
func watched(t *testing.T, fsys *seamtest.MemFS) (app.Model, *seamtest.Watcher) {
	t.Helper()
	w := seamtest.NewWatcher()
	v := watch.New(fsys, "/vault", watch.Options{Debounce: time.Millisecond})
	if err := v.Watch(w); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = v.Close() })
	s, err := config.Startup(fsys, dirs, "/home/u", "/vault/a.md")
	if err != nil {
		t.Fatal(err)
	}
	m, err := app.New(app.Deps{FS: fsys, Watch: v}, "/vault/a.md")
	if err != nil {
		t.Fatal(err)
	}
	return resize(m.WithSession(s), 60, 6), w
}

// settle delivers the next watcher notice to the model, the way the
// program runs the listening command.
func settle(t *testing.T, m app.Model) app.Model {
	t.Helper()
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("no listening command")
	}
	got := make(chan tea.Msg, 1)
	go func() { got <- cmd() }()
	select {
	case msg := <-got:
		next, _ := m.Update(msg)
		return next.(app.Model)
	case <-time.After(2 * time.Second):
		t.Fatal("no notice reached the model")
		return m
	}
}

// change rewrites a.md behind pholio's back and reports it.
func change(t *testing.T, m app.Model, fsys *seamtest.MemFS, w *seamtest.Watcher, text string) app.Model {
	t.Helper()
	if err := fsys.WriteFile("/vault/a.md", []byte(text)); err != nil {
		t.Fatal(err)
	}
	w.Emit(seam.WatchEvent{Path: "/vault/a.md", Op: seam.OpWrite})
	return settle(t, m)
}

func statusLine(m app.Model) string {
	r := rows(m)
	return strings.TrimRight(r[len(r)-2], " ")
}

func text(m app.Model) string { return m.Text() }

// ex types a ":" command and enter.
func ex(m app.Model, cmd string) app.Model {
	m = typeKeys(m, ":"+cmd)
	return press(m, tea.KeyPressMsg{Code: tea.KeyEnter})
}

func TestCleanBufferReloadsSilentlyKeepingTheCursorLine(t *testing.T) {
	fsys := seamtest.NewMemFS(map[string]string{"/vault/a.md": "one\ntwo\nthree\n"})
	m, w := watched(t, fsys)
	m = typeKeys(m, "jj")

	m = change(t, m, fsys, w, "ONE\nTWO\nTHREE\nFOUR\n")

	if got := text(m); got != "ONE\nTWO\nTHREE\nFOUR\n" {
		t.Fatalf("buffer = %q", got)
	}
	if got := statusLine(m); !strings.HasSuffix(got, "3:1") || strings.Contains(got, "[+]") {
		t.Errorf("status line = %q, want clean at 3:1", got)
	}
	if got := messageLine(m); got != "" {
		t.Errorf("message = %q, want a silent reload", got)
	}
	m = typeKeys(m, "u")
	if got := text(m); got != "one\ntwo\nthree\n" {
		t.Errorf("after u buffer = %q, want the reload undone in one step", got)
	}
}

func TestOwnWriteEchoIsNotAChange(t *testing.T) {
	fsys := seamtest.NewMemFS(map[string]string{"/vault/a.md": "one\n"})
	m, w := watched(t, fsys)
	m = typeKeys(m, "x")
	m = ex(m, "w")
	w.Emit(seam.WatchEvent{Path: "/vault/a.md", Op: seam.OpWrite})
	m = change(t, m, fsys, w, "external\n")
	// Had the echo been taken as a change, u would undo to "ne".
	m = typeKeys(m, "u")
	if got := text(m); got != "ne\n" {
		t.Errorf("after u buffer = %q", got)
	}
}

func TestDirtyBufferWarnsAndKeepsEdits(t *testing.T) {
	fsys := seamtest.NewMemFS(map[string]string{"/vault/a.md": "one\n"})
	m, w := watched(t, fsys)
	m = typeKeys(m, "x")

	m = change(t, m, fsys, w, "theirs\n")

	if got := text(m); got != "ne\n" {
		t.Errorf("buffer = %q, want the edit kept", got)
	}
	if got := statusLine(m); !strings.Contains(got, "[changed on disk]") {
		t.Errorf("status line = %q, want a warning", got)
	}
	if got := messageLine(m); !strings.Contains(got, ":e!") {
		t.Errorf("message = %q, want a hint", got)
	}
}

func TestEditBangReloadsFromDisk(t *testing.T) {
	fsys := seamtest.NewMemFS(map[string]string{"/vault/a.md": "one\n"})
	m, w := watched(t, fsys)
	m = typeKeys(m, "x")
	m = change(t, m, fsys, w, "theirs\n")

	m = ex(m, "e!")

	if got := text(m); got != "theirs\n" {
		t.Errorf("buffer = %q", got)
	}
	if got := statusLine(m); strings.Contains(got, "[+]") || strings.Contains(got, "changed") {
		t.Errorf("status line = %q", got)
	}
	// A later write goes straight through.
	m = typeKeys(m, "x")
	_ = ex(m, "w")
	if b, _ := fsys.ReadFile("/vault/a.md"); string(b) != "heirs\n" {
		t.Errorf("file = %q", b)
	}
}

func TestWriteAfterExternalChangeAsksFirst(t *testing.T) {
	for _, c := range []struct {
		key  string
		want string
	}{{"y", "ne\n"}, {"n", "theirs\n"}, {"esc", "theirs\n"}} {
		t.Run(c.key, func(t *testing.T) {
			fsys := seamtest.NewMemFS(map[string]string{"/vault/a.md": "one\n"})
			m, w := watched(t, fsys)
			m = typeKeys(m, "x")
			m = change(t, m, fsys, w, "theirs\n")

			m = ex(m, "w")
			if got := messageLine(m); got != "Changed on disk. Overwrite? y/N" {
				t.Fatalf("message = %q", got)
			}
			if b, _ := fsys.ReadFile("/vault/a.md"); string(b) != "theirs\n" {
				t.Fatalf("written before confirming: %q", b)
			}

			k := tea.KeyPressMsg{Code: rune(c.key[0]), Text: c.key}
			if c.key == "esc" {
				k = tea.KeyPressMsg{Code: tea.KeyEscape}
			}
			m = press(m, k)
			if b, _ := fsys.ReadFile("/vault/a.md"); string(b) != c.want {
				t.Errorf("file = %q, want %q", b, c.want)
			}
			if got := text(m); got != "ne\n" {
				t.Errorf("buffer = %q, the answer key must not reach the editor", got)
			}
			st := statusLine(m)
			if c.key == "y" && (strings.Contains(st, "[+]") || strings.Contains(st, "changed")) {
				t.Errorf("status after overwrite = %q", st)
			}
			if c.key != "y" && !strings.Contains(st, "[+]") {
				t.Errorf("status after cancel = %q, want still dirty", st)
			}
		})
	}
}

func TestDeletedWhileOpen(t *testing.T) {
	fsys := seamtest.NewMemFS(map[string]string{"/vault/a.md": "one\n"})
	m, w := watched(t, fsys)
	if err := fsys.Remove("/vault/a.md"); err != nil {
		t.Fatal(err)
	}
	w.Emit(seam.WatchEvent{Path: "/vault/a.md", Op: seam.OpRemove})
	m = settle(t, m)

	if got := text(m); got != "one\n" {
		t.Errorf("buffer = %q, want it kept", got)
	}
	if got := statusLine(m); !strings.Contains(got, "a.md [deleted]") {
		t.Errorf("status line = %q", got)
	}

	m = ex(m, "w")
	if b, err := fsys.ReadFile("/vault/a.md"); err != nil || string(b) != "one\n" {
		t.Errorf("file = %q, %v; want it written back", b, err)
	}
	if got := statusLine(m); strings.Contains(got, "[deleted]") {
		t.Errorf("status line = %q after writing back", got)
	}
}

func TestFocusRescanPicksUpChanges(t *testing.T) {
	fsys := seamtest.NewMemFS(map[string]string{"/vault/a.md": "one\n"})
	m, err := app.New(app.Deps{FS: fsys}, "/vault/a.md")
	if err != nil {
		t.Fatal(err)
	}
	m = resize(m, 60, 6)
	if !m.View().ReportFocus {
		t.Error("view does not ask for focus reports")
	}
	_ = fsys.WriteFile("/vault/a.md", []byte("two\n"))
	next, _ := m.Update(tea.FocusMsg{})
	if got := text(next.(app.Model)); got != "two\n" {
		t.Errorf("buffer = %q after focus", got)
	}
}

func TestWatchFailureWarns(t *testing.T) {
	fsys := seamtest.NewMemFS(map[string]string{"/vault/a.md": "one\n"})
	m, err := app.New(app.Deps{FS: fsys}, "/vault/a.md")
	if err != nil {
		t.Fatal(err)
	}
	m = resize(m.WithWatchError(errors.New("too many open files")), 80, 6)
	if got := messageLine(m); !strings.Contains(got, "too many open files") || !strings.Contains(got, "focus") {
		t.Errorf("message = %q", got)
	}
}
