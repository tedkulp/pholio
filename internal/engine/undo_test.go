package engine

import "testing"

func TestUndoRedoTable(t *testing.T) {
	runTable(t, []tcase{
		{"undo", "|foo bar", "dwu", "|foo bar"},
		{"undo insert session", "|x", "ihello<esc>u", "|x"},
		{"undo A restores cursor", "a|bc", "Axyz<esc>u", "a|bc"},
		{"undo o", "|a\nb", "oxx<esc>u", "|a\nb"},
		{"undo dd", "a\n|b\nc", "ddu", "a\n|b\nc"},
		{"undo twice", "|a b c", "dwdwuu", "|a b c"},
		{"undo past oldest", "|ab", "xuu", "|ab"},
		{"redo", "|foo bar", "dwu<ctrl+r>", "|bar"},
		{"redo insert", "a|bc", "Axy<esc>u<ctrl+r>", "abcx|y"},
		{"redo past newest", "|ab", "xu<ctrl+r><ctrl+r>", "|b"},
		{"new change clears redo", "|abc", "xux<ctrl+r>", "|bc"},
		{"motion is not a change", "|ab", "lu", "a|b"},
		{"yank is not a change", "|ab cd", "dwywu", "|ab cd"},
	})
	e := load("|a")
	feed(e, "u")
	if e.Msg != "Already at oldest change" {
		t.Errorf("u msg = %q", e.Msg)
	}
	feed(e, "<ctrl+r>")
	if e.Msg != "Already at newest change" {
		t.Errorf("ctrl+r msg = %q", e.Msg)
	}
}

type snap struct {
	text string
	cur  Pos
}

func snapOf(e *Engine) snap { return snap{e.Buf.String(), e.Cur} }

// TestUndoRedoRestoresExactState applies a series of changes, then checks
// that each undo gives back the exact buffer and cursor from before the
// change, and each redo the exact state after it.
func TestUndoRedoRestoresExactState(t *testing.T) {
	// Each entry is one change, or a motion (which changes nothing and is
	// skipped). "2ddu<ctrl+r>" nets out to the one change made by 2dd.
	cmds := []string{
		"w", "dw", "j", "A tail<esc>", "f(", "di(", "0", "ct.x<esc>",
		"gg", "Otop<esc>", "G", "dd", "3x",
		"kkk", "ciwNEW<esc>", "yy", "j", "p", "k", "J", "0", "~", "rZ", "2ddu<ctrl+r>",
		"$", "i<enter><esc>", "gg", "dap", "o- [ ] task<enter>next<esc>",
		"x", ".",
	}
	e := load("|one two three\n  four (five) six.\nseven\n\neight nine\n- item\n(a b)")
	var before, after []snap
	for _, c := range cmds {
		b := snapOf(e)
		feed(e, c)
		if e.Mode != Normal || e.PendingKeys() != "" {
			t.Fatalf("%q left mode %v pending %q", c, e.Mode, e.PendingKeys())
		}
		if a := snapOf(e); a.text != b.text {
			before, after = append(before, b), append(after, a)
		}
	}
	if len(before) < 15 {
		t.Fatalf("expected at least 15 changes, got %d", len(before))
	}
	for i := len(before) - 1; i >= 0; i-- {
		feed(e, "u")
		if got := snapOf(e); got != before[i] {
			t.Fatalf("undo %d:\n got %+v\nwant %+v", i, got, before[i])
		}
	}
	for i := range after {
		feed(e, "<ctrl+r>")
		if got := snapOf(e); got != after[i] {
			t.Fatalf("redo %d:\n got %+v\nwant %+v", i, got, after[i])
		}
	}
}
