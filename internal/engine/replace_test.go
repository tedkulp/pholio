package engine

import "testing"

func TestReplaceIsAnUnsavedUndoStep(t *testing.T) {
	e := load("one\ntw|o")
	e.Replace("ONE\nTWO\n")
	if got, want := show(e), "ONE\nTW|O"; got != want {
		t.Fatalf("after replace:\n%s\nwant\n%s", got, want)
	}
	if !e.Dirty {
		t.Error("replace left the buffer clean; its text is not on disk")
	}
	feed(e, "u")
	if got, want := show(e), "one\ntw|o"; got != want {
		t.Fatalf("after undo:\n%s\nwant\n%s", got, want)
	}
	if e.Dirty {
		t.Error("undoing the replace back to the loaded text left it dirty")
	}
}
