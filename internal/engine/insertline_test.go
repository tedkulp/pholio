package engine

import "testing"

func TestInsertLineKeepsTheCursorOnItsTextAndIsOneUndoStep(t *testing.T) {
	e := load("a\nb|c\nd")
	e.InsertLine(1, "new")
	if got, want := show(e), "a\nnew\nb|c\nd"; got != want {
		t.Fatalf("after insert:\n%s\nwant\n%s", got, want)
	}
	if !e.Dirty {
		t.Error("insert left the buffer clean")
	}
	feed(e, "u")
	if got := e.Buf.String(); got != "a\nbc\nd\n" {
		t.Fatalf("after undo: %q", got)
	}
}

func TestInsertLineAtTheEnd(t *testing.T) {
	e := load("|a\nb")
	e.InsertLine(2, "new")
	if got, want := show(e), "|a\nb\nnew"; got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}
