package engine

import "testing"

func TestVersionChangesWithEveryEditAndNotOnMotions(t *testing.T) {
	e := load("|one\ntwo")
	v0 := e.Buf.Version()
	for _, k := range keys("jw0") {
		e.Feed(k)
	}
	if e.Buf.Version() != v0 {
		t.Fatal("a motion changed the version")
	}
	seen := map[uint64]bool{v0: true}
	for _, step := range []string{"x", "u", "<ctrl+r>", "ihi<esc>"} {
		for _, k := range keys(step) {
			e.Feed(k)
		}
		if seen[e.Buf.Version()] {
			t.Fatalf("after %q the version was seen before", step)
		}
		seen[e.Buf.Version()] = true
	}
	e.Reload("other\n")
	if seen[e.Buf.Version()] {
		t.Fatal("Reload kept an old version")
	}
}
