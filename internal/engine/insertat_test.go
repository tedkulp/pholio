package engine

import "testing"

func TestInsertAfterCursorIsOneUndoStep(t *testing.T) {
	e := load("See| \nnext")
	e.InsertAfterCursor("[[z]]")
	if got, want := show(e), "See [[z]|]\nnext"; got != want {
		t.Fatalf("after insert:\n%s\nwant\n%s", got, want)
	}
	if !e.Dirty {
		t.Error("insert left the buffer clean")
	}
	feed(e, "u")
	if got, want := show(e), "See| \nnext"; got != want {
		t.Fatalf("after undo:\n%s\nwant\n%s", got, want)
	}
}

func TestInsertAfterCursorOnAnEmptyLine(t *testing.T) {
	e := load("a\n|\nb")
	e.InsertAfterCursor("[[z]]")
	if got, want := show(e), "a\n[[z]|]\nb"; got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}
