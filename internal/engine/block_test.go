package engine

import "testing"

func TestBlockwiseVisual(t *testing.T) {
	runTable(t, []tcase{
		{"d", "|abcd\nabcd\nabcd", "<ctrl+v>jjld", "|cd\ncd\ncd"},
		{"x", "a|bcd\nabcd", "<ctrl+v>jx", "a|cd\nacd"},
		{"d backward", "abcd\nab|cd", "<ctrl+v>khd", "a|d\nad"},
		{"d skips a short line", "a|bc\nx\nabc", "<ctrl+v>jjld", "|a\nx\na"},
		{"short line keeps the remembered column", "a\nab|c", "<ctrl+v>kd", "|a\nab"},
		{"short line between keeps the remembered column", "a|bc\nx\nabc", "<ctrl+v>jd", "a|c\nx\nabc"},
		{"tab and spaces are the same cells", "|\tabc\n    abc", "<ctrl+v>jd", "|abc\nabc"},
		{"wide glyph straddling the edge is inside", "a|bcd\n日xyz\nabcd", "<ctrl+v>jjd", "a|cd\nxyz\nacd"},
		{"$ d", "a|bc\na\nabcd", "<ctrl+v>$jjd", "|a\na\na"},
		{"$ ends on another motion", "|ab\nabcd", "<ctrl+v>j$hA;<esc>", "ab |;\nabc;d"},
		{"I", "|a\nb\nc", "<ctrl+v>jjI- <esc>", "|- a\n- b\n- c"},
		{"I skips short lines", "a|bc\nx\nabc", "<ctrl+v>jjIX<esc>", "a|Xbc\nx\naXbc"},
		{"I with a newline keeps the first line only", "|a\nb", "<ctrl+v>jIx<enter>y<esc>", "x\n|ya\nb"},
		{"I left with ctrl+c keeps the first line only", "|a\nb", "<ctrl+v>jIx<ctrl+c>", "|xa\nb"},
		{"A after $", "|ab\nabcd", "<ctrl+v>j$A;<esc>", "ab|;\nabcd;"},
		{"A pads short lines", "|abc\na\nabc", "<ctrl+v>lljjA;<esc>", "abc|;\na  ;\nabc;"},
		{"c", "|abcd\nabcd", "<ctrl+v>jlcXY<esc>", "|XYcd\nXYcd"},
		{"c on whole lines", "|ab\nab", "<ctrl+v>jlcX<esc>", "|X\nX"},
		{">", "|a\nb\nc", "<ctrl+v>jj<gt>", "\t|a\n\tb\n\tc"},
		{"<", "\t|a\n\tb\n\tc", "<ctrl+v>jj<lt>", "|a\nb\nc"},
		{"~", "|abc\nabc", "<ctrl+v>jl~", "|ABc\nABc"},
		{"y moves to the top-left", "abc\na|bc", "<ctrl+v>khyx", "|bc\nabc"},
		{"y p", "|abc\nxyz\n12\n3", "<ctrl+v>jlyjjlp", "abc\nxyz\n12|ab\n3 xy"},
		{"y P", "|ab\ncd\n\nxy", "<ctrl+v>jlyjjjP", "ab\ncd\n\n|abxy\ncd"},
		{"p adds lines at the end", "|ab\ncd\nx", "<ctrl+v>jlyGp", "ab\ncd\nx|ab\n cd"},
		{"P pads a ragged block", "|a\nbc\nxy\nzw", "<ctrl+v>j$yjjP", "a\nbc\n|a xy\nbczw"},
		{"register", "|ab\ncd", `<ctrl+v>j"ayx"aP`, "|ab\nccd"},
		{"o swaps the corners", "|abc\nabc", "<ctrl+v>jlold", "a|c\nac"},
		{"J", "|a\nb\nc", "<ctrl+v>jJ", "a| b\nc"},
		{"esc leaves", "|ab\nab", "<ctrl+v>j<esc>x", "ab\n|b"},
		{"ctrl+v toggles off", "|ab\nab", "<ctrl+v>j<ctrl+v>x", "ab\n|b"},
	})
}

func TestBlockwiseUndo(t *testing.T) {
	for _, keys := range []string{
		"<ctrl+v>jld", "<ctrl+v>jIX<esc>", "<ctrl+v>j$A;<esc>", "<ctrl+v>lljA;<esc>",
		"<ctrl+v>jlcX<esc>", "<ctrl+v>j<gt>", "<ctrl+v>jl~", "<ctrl+v>jlyGp",
	} {
		e := load("|abc\na\nabc")
		feed(e, keys)
		feed(e, "u")
		if g := e.Buf.String(); g != "abc\na\nabc\n" {
			t.Errorf("%q then u: %q", keys, g)
		}
	}
}

func TestBlockwiseModes(t *testing.T) {
	e := load("|abc\nabc")
	feed(e, "v<ctrl+v>")
	if e.Mode != VisualBlock || e.Mode.String() != "V-BLOCK" {
		t.Fatalf("v ctrl+v: mode %v", e.Mode)
	}
	feed(e, "V")
	if e.Mode != VisualLine {
		t.Errorf("ctrl+v V: mode %v", e.Mode)
	}
	feed(e, "<ctrl+v><ctrl+v>")
	if e.Mode != Normal {
		t.Errorf("ctrl+v ctrl+v: mode %v", e.Mode)
	}
	feed(e, "<ctrl+v>jd")
	if e.Mode != Normal {
		t.Errorf("after d: mode %v", e.Mode)
	}
}

func TestBlockwiseSelectedRange(t *testing.T) {
	e := load("|\tabc\n    abc\n\nab")
	feed(e, "<ctrl+v>jjjl")
	want := []struct {
		from, to int
		ok       bool
	}{{0, 1, true}, {0, 4, true}, {0, 0, false}, {0, 2, true}}
	for i, w := range want {
		from, to, ok := e.SelectedRange(i)
		if from != w.from || to != w.to || ok != w.ok {
			t.Errorf("line %d: %d %d %v, want %+v", i, from, to, ok, w)
		}
	}
}

func TestReloadEndsABlockInsert(t *testing.T) {
	e := load("|a\nb")
	feed(e, "<ctrl+v>jIx")
	e.Reload("a\nb")
	feed(e, "iy<esc>")
	if g := e.Buf.String(); g != "ya\nb\n" {
		t.Errorf("a later insert was copied to the old block: %q", g)
	}
}
