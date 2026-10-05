package engine

import (
	"slices"
	"strings"
	"testing"
)

// spy is a Fixup that upper-cases every line the change touched, so tests
// can see when it ran and what it was given.
type spy struct {
	calls    int
	before   []string
	after    []string
	inserted bool
}

func (s *spy) hook(e *Engine, before, after []string, inserted bool) {
	s.calls++
	s.before, s.after, s.inserted = before, slices.Clone(after), inserted
	for i, l := range after {
		if i >= len(before) || before[i] != l {
			e.SetLine(i, strings.ToUpper(l))
		}
	}
}

func TestFixupSeesTheBufferBeforeAndAfterANormalCommand(t *testing.T) {
	e := load("|ab\ncd")
	s := &spy{}
	e.Fixup = s.hook

	feed(e, "x")

	if s.calls != 1 {
		t.Fatalf("Fixup ran %d times, want 1", s.calls)
	}
	if !slices.Equal(s.before, []string{"ab", "cd"}) || !slices.Equal(s.after, []string{"b", "cd"}) {
		t.Errorf("before %q after %q", s.before, s.after)
	}
	if s.inserted {
		t.Error("inserted is set for x")
	}
	if g := show(e); g != "|B\ncd" {
		t.Errorf("buffer %q", g)
	}
}

func TestFixupRunsOnceOnLeavingInsertMode(t *testing.T) {
	e := load("|ab")
	s := &spy{}
	e.Fixup = s.hook

	feed(e, "Ayz")
	if s.calls != 0 {
		t.Fatalf("Fixup ran %d times in insert mode, want 0", s.calls)
	}
	feed(e, "<enter>q<esc>")

	if s.calls != 1 || !s.inserted {
		t.Fatalf("Fixup ran %d times, inserted %v; want once, inserted", s.calls, s.inserted)
	}
	if g := show(e); g != "ABYZ\n|Q" {
		t.Errorf("buffer %q", g)
	}
}

func TestFixupDoesNotRunWithoutAChange(t *testing.T) {
	e := load("|ab\ncd")
	s := &spy{}
	e.Fixup = s.hook

	feed(e, "jlyyi<esc>")

	if s.calls != 0 {
		t.Errorf("Fixup ran %d times, want 0", s.calls)
	}
}

func TestFixupEditsAreUndoneAndRedoneWithTheChange(t *testing.T) {
	e := load("|ab\ncd")
	s := &spy{}
	e.Fixup = s.hook

	feed(e, "x")
	feed(e, "u")
	if g := show(e); g != "|ab\ncd" {
		t.Fatalf("after u: %q", g)
	}
	if s.calls != 1 {
		t.Errorf("Fixup ran %d times, want 1 (not for undo)", s.calls)
	}
	feed(e, "<ctrl+r>")
	if g := show(e); g != "|B\ncd" {
		t.Errorf("after ctrl+r: %q", g)
	}
}

func TestSetLineOutsideACommandIsItsOwnUndoStep(t *testing.T) {
	e := load("ab\nc|d")

	e.SetLine(0, "xyz")
	if g := show(e); g != "xyz\nc|d" || !e.Dirty {
		t.Fatalf("after SetLine: %q dirty %v", g, e.Dirty)
	}
	feed(e, "x")
	feed(e, "u")
	if g := show(e); g != "xyz\nc|d" {
		t.Fatalf("first u: %q", g)
	}
	feed(e, "u")
	if g := show(e); g != "ab\nc|d" {
		t.Errorf("second u: %q", g)
	}
}

func TestSetLineKeepsTheCursorOnTheLine(t *testing.T) {
	e := load("abcdef|g")

	e.SetLine(0, "ab")

	if g := show(e); g != "a|b" {
		t.Errorf("got %q", g)
	}
}
