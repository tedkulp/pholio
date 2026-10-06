package app_test

import (
	"bytes"
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
// the screen as plain text. Like a tty with ONLCR set, it turns each "\n"
// the renderer writes into "\r\n".
func (s *vtScreen) update(out []byte) string {
	_, _ = s.emu.Write(bytes.ReplaceAll(out[s.seen:], []byte("\n"), []byte("\r\n")))
	s.seen = len(out)
	return s.emu.String()
}

// waitFor waits until the screen satisfies cond. Each teatest.WaitFor
// reads only output not consumed by an earlier one, so the slice it hands
// over starts afresh.
func (s *vtScreen) waitFor(t *testing.T, tm *teatest.TestModel, cond func(screen string) bool) {
	t.Helper()
	s.seen = 0
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return cond(s.update(out))
	}, teatest.WithDuration(3*time.Second))
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

func TestSmokeTypingEditsTheNote(t *testing.T) {
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

	tm.Type("dwiHello ")
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEsc})
	// The edited line itself is checked on the final model, not the screen:
	// the renderer may draw an insertion with the terminal's insert mode
	// (CSI 4 h), which x/vt doesn't emulate, so the screen can show
	// "HelloWelcome" depending on how frames were batched.
	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return strings.Contains(scr.update(out), "NORMAL  README.md [+]")
	}, teatest.WithDuration(3*time.Second))

	tm.Send(tea.KeyPressMsg{Code: 'q', Mod: tea.ModCtrl})
	scr.waitFor(t, tm, func(s string) bool {
		return strings.Contains(s, "Save changes to README.md before quitting?")
	})
	tm.Send(tea.KeyPressMsg{Code: 'n', Text: "n"})
	final, ok := tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second)).(app.Model)
	if !ok {
		t.Fatal("final model is not an app.Model")
	}
	if got := final.Text(); !strings.HasPrefix(got, "Hello Welcome to the basic Vault") {
		t.Fatalf("buffer starts %q, want the edit applied", strings.SplitN(got, "\n", 2)[0])
	}
}
