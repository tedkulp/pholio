package app_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/tedkulp/pholio/internal/app"
	"github.com/tedkulp/pholio/internal/seam"
	"github.com/tedkulp/pholio/internal/seam/seamtest"
)

func TestCtrlQQuitsFromAMessageLinePrompt(t *testing.T) {
	m := ex(shell(t, shellVault(), "/vault/a.md"), "delete")
	if got := messageLine(m); !strings.HasPrefix(got, "Delete a.md") {
		t.Fatalf("message = %q, want the delete prompt", got)
	}

	_, cmd := m.Update(ctrlQ)

	if !quits(cmd) {
		t.Error("ctrl+q at a prompt did not quit")
	}
}

func TestCtrlQAtAPromptStillAsksAboutUnsavedChanges(t *testing.T) {
	m := ex(typeKeys(shell(t, shellVault(), "/vault/a.md"), "x"), "delete")

	next, cmd := m.Update(ctrlQ)

	if cmd != nil {
		t.Fatal("ctrl+q on a dirty buffer quit without asking")
	}
	if got := messageLine(next.(app.Model)); !strings.HasPrefix(got, "Save changes to a.md before quitting?") {
		t.Errorf("message = %q, want the quit question", got)
	}
}

func TestADeletedBufferCountsAsUnsaved(t *testing.T) {
	fsys := seamtest.NewMemFS(map[string]string{"/vault/a.md": "one\n"})
	m, w := watched(t, fsys)
	if err := fsys.Remove("/vault/a.md"); err != nil {
		t.Fatal(err)
	}
	w.Emit(seam.WatchEvent{Path: "/vault/a.md", Op: seam.OpRemove})
	m = settle(t, m)

	next, cmd := m.Update(ctrlQ)

	if cmd != nil {
		t.Fatal("ctrl+q on a [deleted] buffer quit without asking")
	}
	m = next.(app.Model)
	if got := messageLine(m); !strings.HasPrefix(got, "Save changes to a.md before quitting?") {
		t.Fatalf("message = %q, want the quit question", got)
	}
	_, cmd = m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	if !quits(cmd) {
		t.Error("y did not quit")
	}
	if b, err := fsys.ReadFile("/vault/a.md"); err != nil || string(b) != "one\n" {
		t.Errorf("a.md = %q, %v; want it written back", b, err)
	}
}

func TestWqQuitsOnceTheOverwriteIsConfirmed(t *testing.T) {
	fsys := seamtest.NewMemFS(map[string]string{"/vault/a.md": "one\n"})
	m, w := watched(t, fsys)
	m = typeKeys(m, "x")
	m = change(t, m, fsys, w, "theirs\n")
	m = ex(m, "wq")
	if got := messageLine(m); got != "Changed on disk. Overwrite? y/N" {
		t.Fatalf("message = %q", got)
	}

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})

	if !quits(cmd) {
		t.Error(":wq did not quit after the overwrite was confirmed")
	}
	if b, _ := fsys.ReadFile("/vault/a.md"); string(b) != "ne\n" {
		t.Errorf("file = %q", b)
	}
}

func TestWAfterOverwriteDoesNotQuit(t *testing.T) {
	fsys := seamtest.NewMemFS(map[string]string{"/vault/a.md": "one\n"})
	m, w := watched(t, fsys)
	m = typeKeys(m, "x")
	m = change(t, m, fsys, w, "theirs\n")
	m = ex(m, "w")

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})

	if quits(cmd) {
		t.Error(":w quit after the overwrite was confirmed")
	}
}
