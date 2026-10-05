package engine

import "testing"

func TestReloadKeepsCursorLineAndIsOneUndoStep(t *testing.T) {
	e := load("one\ntwo\nthr|ee\nfour")
	e.Reload("ONE\nTWO\nTHREE!\nFOUR\nFIVE\n")
	if got, want := show(e), "ONE\nTWO\nTHR|EE!\nFOUR\nFIVE"; got != want {
		t.Fatalf("after reload:\n%s\nwant\n%s", got, want)
	}
	if e.Dirty {
		t.Error("reload left the buffer dirty")
	}
	feed(e, "u")
	if got, want := show(e), "one\ntwo\nthr|ee\nfour"; got != want {
		t.Fatalf("after undo:\n%s\nwant\n%s", got, want)
	}
	if !e.Dirty {
		t.Error("undoing a reload should leave the buffer differing from disk (dirty)")
	}
	feed(e, "<ctrl+r>")
	if got, want := show(e), "ONE\nTWO\nTHR|EE!\nFOUR\nFIVE"; got != want {
		t.Fatalf("after redo:\n%s\nwant\n%s", got, want)
	}
}

func TestReloadClampsCursor(t *testing.T) {
	e := load("a\nb\nc\nlong li|ne")
	e.Reload("x\nyy\n")
	if got, want := show(e), "x\ny|y"; got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestReloadSameTextIsNoChange(t *testing.T) {
	e := load("a|b")
	e.Reload("ab\n")
	feed(e, "u")
	if e.Msg != "Already at oldest change" {
		t.Errorf("identical reload recorded an undo step (msg %q)", e.Msg)
	}
}

func TestReloadLeavesInsertMode(t *testing.T) {
	e := load("|ab")
	feed(e, "i")
	e.Reload("cd\n")
	if e.Mode != Normal {
		t.Errorf("mode = %v, want NORMAL", e.Mode)
	}
	feed(e, "u")
	if got := show(e); got != "|ab" {
		t.Errorf("after undo got %q", got)
	}
}
