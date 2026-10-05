package app_test

import (
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/exp/teatest/v2"
	"github.com/charmbracelet/x/vt"

	"github.com/tedkulp/pholio/internal/app"
	"github.com/tedkulp/pholio/internal/seam"
	"github.com/tedkulp/pholio/internal/testutil"
)

const smokeW, smokeH = 60, 12

// vtScreen replays a program's raw output through a virtual terminal so
// tests can assert on what is visible rather than on the renderer's
// incremental byte stream.
type vtScreen struct {
	emu  *vt.Emulator
	seen int
}

func newVTScreen() *vtScreen {
	emu := vt.NewEmulator(smokeW, smokeH)
	// The emulator answers terminal queries on its input pipe; drain it so
	// those writes never block. Emulator.Close races with Read, so the
	// drain goroutine is left to end with the test binary.
	go func() { _, _ = io.Copy(io.Discard, emu) }()
	return &vtScreen{emu: emu}
}

// update feeds the not-yet-seen tail of out (all output so far) and returns
// the screen as plain text.
func (s *vtScreen) update(out []byte) string {
	_, _ = s.emu.Write(out[s.seen:])
	s.seen = len(out)
	return s.emu.String()
}

func TestSmokeStartShowsNoteAndCtrlQQuits(t *testing.T) {
	vault := testutil.CopyVault(t, "basic")
	m, err := app.New(app.Deps{FS: seam.OSFS{}, Clock: seam.SystemClock{}}, filepath.Join(vault, "README.md"))
	if err != nil {
		t.Fatal(err)
	}

	tm := teatest.NewTestModel(t, m,
		teatest.WithInitialTermSize(smokeW, smokeH),
		teatest.WithProgramOptions(tea.WithColorProfile(colorprofile.Ascii)),
	)
	scr := newVTScreen()
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		s := scr.update(out)
		return strings.Contains(s, "# Welcome to the basic Vault") && strings.Contains(s, "README.md")
	}, teatest.WithDuration(3*time.Second))

	tm.Send(tea.KeyPressMsg{Code: 'q', Mod: tea.ModCtrl})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}
