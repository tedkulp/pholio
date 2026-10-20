package engine

import (
	"errors"
	"slices"
	"testing"

	"github.com/tedkulp/pholio/internal/seam/seamtest"
)

// loadClip is load with a fake clipboard attached.
func loadClip(in string) (*Engine, *seamtest.Clipboard) {
	e := load(in)
	cb := &seamtest.Clipboard{}
	e.Clipboard = cb
	return e, cb
}

func TestYankWritesClipboard(t *testing.T) {
	for _, c := range []struct{ name, in, keys, want string }{
		{"yy", "|ab\ncd", "yy", "ab\n"},
		{"dw", "|ab cd", "dw", "ab "},
		{"visual y", "|abc", "vly", "ab"},
		{"x", "|abc", "x", "a"},
		{"cw", "|ab cd", "cwX<esc>", "ab"},
		{"block y", "|ab\ncd", "<ctrl+v>jy", "a\nc"},
		{"named register", "|ab\ncd", `"ayy`, "ab\n"},
		{"plus register", "|ab\ncd", `"+yy`, "ab\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			e, cb := loadClip(c.in)
			feed(e, c.keys)
			if w := cb.Writes(); len(w) != 1 || w[0] != c.want {
				t.Errorf("writes = %q, want [%q]", w, c.want)
			}
		})
	}
}

func TestPasteFromClipboard(t *testing.T) {
	for _, c := range []struct{ name, in, before, clip, keys, want string }{
		{"inline after cursor", "|ab", "", "XY", "p", "aX|Yb"},
		{"inline before cursor", "a|b", "", "XY", "P", "aX|Yb"},
		{"lines below", "|ab\ncd", "", "X\nY\n", "p", "ab\n|X\nY\ncd"},
		{"lines above", "ab\n|cd", "", "X\n", "P", "ab\n|X\ncd"},
		{"CRLF lines", "|ab", "", "X\r\nY\r\n", "p", "ab\n|X\nY"},
		{"plus register", "|ab", "", "XY", `"+p`, "aX|Yb"},
		{"star register", "|ab", "", "XY", `"*p`, "aX|Yb"},
		{"explicit unnamed", "|ab", "", "XY", `""p`, "aX|Yb"},
		{"copied after a yank", "|ab", "yl", "XY", "p", "aX|Yb"},
	} {
		t.Run(c.name, func(t *testing.T) {
			e, cb := loadClip(c.in)
			feed(e, c.before)
			cb.Set(c.clip) // another app copies
			feed(e, c.keys)
			if g := show(e); g != c.want {
				t.Errorf("got %q, want %q", g, c.want)
			}
		})
	}
}

func TestPasteKeepsShapeOfOwnClipboard(t *testing.T) {
	// Blockwise: the clipboard holds "ab\nxy" (no trailing newline), which
	// read cold would paste inline. pholio wrote it, so the block wins.
	e, _ := loadClip("|abc\nxyz\n12\n3")
	feed(e, "<ctrl+v>jlyjjlp")
	if g, want := show(e), "abc\nxyz\n12|ab\n3 xy"; g != want {
		t.Errorf("block: got %q, want %q", g, want)
	}
	// Linewise yank of a line pastes below even with p at line end.
	e, _ = loadClip("|a\nb")
	feed(e, "yyjp")
	if g, want := show(e), "a\nb\n|a"; g != want {
		t.Errorf("line: got %q, want %q", g, want)
	}
}

func TestNamedRegisterIgnoresClipboard(t *testing.T) {
	e, cb := loadClip("|a\nb")
	feed(e, `"ayyj`)
	cb.Set("other")
	feed(e, `"ap`)
	if g, want := show(e), "a\nb\n|a"; g != want {
		t.Errorf("got %q, want %q", g, want)
	}
	// "0 is internal too.
	feed(e, `"0p`)
	if g, want := show(e), "a\nb\na\n|a"; g != want {
		t.Errorf(`"0p: got %q, want %q`, g, want)
	}
	if w := cb.Writes(); !slices.Equal(w, []string{"a\n"}) {
		t.Errorf("writes = %q", w)
	}
}

func TestPlusRegisterIsUnnamed(t *testing.T) {
	// "+yy stores like an unnamed yank: p (no register) gets it back,
	// with the clipboard broken too.
	e, cb := loadClip("|a\nb")
	cb.ReadErr = errors.New("no clipboard")
	feed(e, `"+yyjp`)
	if g, want := show(e), "a\nb\n|a"; g != want {
		t.Errorf("got %q, want %q", g, want)
	}
}

func TestClipboardErrors(t *testing.T) {
	e, cb := loadClip("|ab\ncd")
	cb.ReadErr = errors.New("read failed")
	cb.WriteErr = errors.New("write failed")
	feed(e, "yyjp")
	if g, want := show(e), "ab\ncd\n|ab"; g != want {
		t.Errorf("p: got %q, want %q", g, want)
	}
	feed(e, `"+P`)
	if g, want := show(e), "ab\ncd\n|ab\nab"; g != want {
		t.Errorf(`"+P: got %q, want %q`, g, want)
	}
	if e.Msg != "" {
		t.Errorf("msg = %q, want none", e.Msg)
	}
}

func TestEmptyClipboardUsesRegister(t *testing.T) {
	e, cb := loadClip("|ab")
	feed(e, "yl")
	cb.Set("")
	feed(e, "p")
	if g, want := show(e), "a|ab"; g != want {
		t.Errorf("got %q, want %q", g, want)
	}
}

func TestNothingToPaste(t *testing.T) {
	e, _ := loadClip("|ab")
	feed(e, "p")
	if g := show(e); g != "|ab" {
		t.Errorf("got %q", g)
	}
	if e.Msg != `E353: Nothing in register "` {
		t.Errorf("msg = %q", e.Msg)
	}
}

func TestDotRepeatsClipboardPaste(t *testing.T) {
	e, cb := loadClip("|ab")
	cb.Set("X")
	feed(e, "p.")
	if g, want := show(e), "aX|Xb"; g != want {
		t.Errorf("got %q, want %q", g, want)
	}
}

func TestFailedWriteDoesNotPasteOldClipboard(t *testing.T) {
	// The write fails but the read works: the clipboard still holds the
	// old text, so p must use what was just yanked.
	e, cb := loadClip("|ab")
	cb.Set("old")
	cb.WriteErr = errors.New("timed out")
	feed(e, "ylp")
	if g, want := show(e), "a|ab"; g != want {
		t.Errorf("got %q, want %q", g, want)
	}
	// Once a write succeeds, the clipboard is read again.
	cb.WriteErr = nil
	feed(e, "yl")
	cb.Set("X")
	feed(e, "p")
	if g, want := show(e), "aa|Xb"; g != want {
		t.Errorf("after recovery: got %q, want %q", g, want)
	}
}
