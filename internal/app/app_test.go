package app_test

import (
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"

	"github.com/tedkulp/pholio/internal/app"
	"github.com/tedkulp/pholio/internal/seam"
	"github.com/tedkulp/pholio/internal/seam/seamtest"
	"github.com/tedkulp/pholio/internal/testutil"
)

func TestViewShowsTheNote(t *testing.T) {
	vault := testutil.CopyVault(t, "basic")

	m, err := app.New(app.Deps{FS: seam.OSFS{}}, filepath.Join(vault, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	next, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 10})

	golden.RequireEqual(t, ansi.Strip(next.View().Content))
}

func TestCtrlQQuits(t *testing.T) {
	fsys := seamtest.NewMemFS(map[string]string{"/vault/a.md": "hello\n"})
	m, err := app.New(app.Deps{FS: fsys, Clock: seamtest.NewClock(time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC))}, "/vault/a.md")
	if err != nil {
		t.Fatal(err)
	}

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'q', Mod: tea.ModCtrl})

	if cmd == nil {
		t.Fatal("ctrl+q returned no command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("ctrl+q command produced %T, want tea.QuitMsg", cmd())
	}
}

func TestMissingFileOpensEmpty(t *testing.T) {
	fsys := seamtest.NewMemFS(nil)

	m, err := app.New(app.Deps{FS: fsys}, "/vault/new.md")
	if err != nil {
		t.Fatalf("opening a missing Note: %v", err)
	}
	next, _ := m.Update(tea.WindowSizeMsg{Width: 20, Height: 3})

	want := "\n\n new.md             " // the status line fills the width
	if got := ansi.Strip(next.View().Content); got != want {
		t.Fatalf("view = %q, want %q", got, want)
	}
}

func TestUnreadableFileIsAnError(t *testing.T) {
	// A directory where the Note should be cannot be read as a file.
	fsys := seamtest.NewMemFS(map[string]string{"/vault/dir/x.md": ""})

	if _, err := app.New(app.Deps{FS: fsys}, "/vault/dir"); err == nil {
		t.Fatal("expected an error opening a directory as a Note")
	}
}
