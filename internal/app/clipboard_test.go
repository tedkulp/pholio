package app_test

import (
	"reflect"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/tedkulp/pholio/internal/app"
	"github.com/tedkulp/pholio/internal/seam"
	"github.com/tedkulp/pholio/internal/seam/seamtest"
)

func clipApp(t *testing.T, cb *seamtest.Clipboard) app.Model {
	t.Helper()
	fsys := seamtest.NewMemFS(map[string]string{"/vault/a.md": "hello\n", "/vault/b.md": "bye\n"})
	m, err := app.New(app.Deps{FS: fsys, Clipboard: cb}, "/vault/a.md")
	if err != nil {
		t.Fatal(err)
	}
	return resize(m, 80, 12)
}

// msgs runs cmd and returns the messages it gives, batches flattened.
func msgs(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, msgs(c)...)
		}
		return out
	}
	return []tea.Msg{msg}
}

func setsClipboard(cmd tea.Cmd, text string) bool {
	want := tea.SetClipboard(text)()
	for _, msg := range msgs(cmd) {
		if reflect.DeepEqual(msg, want) {
			return true
		}
	}
	return false
}

func TestYankAndPutUseTheClipboard(t *testing.T) {
	cb := &seamtest.Clipboard{}
	m := clipApp(t, cb)
	m, cmd := pressing(typeKeys(m, "y"), tea.KeyPressMsg{Code: 'y', Text: "y"})
	if w := cb.Writes(); len(w) != 1 || w[0] != "hello\n" {
		t.Errorf("writes = %q", w)
	}
	if setsClipboard(cmd, "hello\n") {
		t.Error("OSC 52 sent although the clipboard tool worked")
	}
	cb.Set("X")
	if m = typeKeys(m, "P"); m.Text() != "Xhello\n" {
		t.Errorf("text = %q", m.Text())
	}
}

func TestOpenedNoteUsesTheClipboard(t *testing.T) {
	cb := &seamtest.Clipboard{}
	m := clipApp(t, cb)
	m = typeKeys(m, ":e /vault/b.md")
	m = press(m, enter)
	cb.Set("X")
	if m = typeKeys(m, "P"); m.Text() != "Xbye\n" {
		t.Errorf("text = %q", m.Text())
	}
}

func TestNoClipboardToolFallsBackToOSC52(t *testing.T) {
	cb := &seamtest.Clipboard{WriteErr: seam.ErrNoClipboard, ReadErr: seam.ErrNoClipboard}
	m := clipApp(t, cb)
	m, cmd := pressing(typeKeys(m, "y"), tea.KeyPressMsg{Code: 'y', Text: "y"})
	if !setsClipboard(cmd, "hello\n") {
		t.Errorf("no OSC 52 write in %#v", msgs(cmd))
	}
	// A put uses the register; the message line stays empty.
	m, cmd = pressing(m, tea.KeyPressMsg{Code: 'p', Text: "p"})
	if m.Text() != "hello\nhello\n" {
		t.Errorf("text = %q", m.Text())
	}
	if setsClipboard(cmd, "hello\n") {
		t.Error("OSC 52 sent again by a put")
	}
}
