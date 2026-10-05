package engine

import "testing"

func TestInsertMode(t *testing.T) {
	runTable(t, []tcase{
		{"i", "a|c", "ib<esc>", "a|bc"},
		{"a", "a|c", "ab<esc>", "ac|b"},
		{"a on empty line", "|", "ax<esc>", "|x"},
		{"I", "  ab|c", "Ix<esc>", "  |xabc"},
		{"A", "a|bc", "Ax<esc>", "abc|x"},
		{"o", "|  a\nb", "ox<esc>", "  a\n  |x\nb"},
		{"O", "  |a", "Ox<esc>", "  |x\n  a"},
		{"esc moves left", "|", "iabc<esc>", "ab|c"},
		{"ctrl+c leaves insert", "|", "iab<ctrl+c>", "a|b"},
		{"multibyte text", "|", "iЖ👍🏽<esc>", "Ж|👍🏽"},
		{"enter splits", "ab|cd", "i<enter><esc>", "ab\n|cd"},
		{"enter keeps indent", "  ab|", "a<enter>x<esc>", "  ab\n  |x"},
		{"backspace", "ab|c", "i<backspace><esc>", "|ac"},
		{"backspace joins lines", "a\n|b", "i<backspace><esc>", "|ab"},
		{"backspace grapheme", "x👍🏽|y", "i<backspace><esc>", "|xy"},
		{"delete", "a|bc", "i<delete><esc>", "|ac"},
		{"delete joins lines", "a|\nb", "a<delete><esc>", "|ab"},
		{"ctrl+w", "foo bar|", "a<ctrl+w><esc>", "foo| "},
		{"ctrl+w punct", "foo.bar|", "a<ctrl+w><ctrl+w><esc>", "fo|o"},
		{"ctrl+u", "  foo bar|", "a<ctrl+u><esc>", " | "},
		{"ctrl+u at indent", "  |foo", "i<ctrl+u><esc>", "|foo"},
		{"tab", "|a", "i<tab><esc>", "|\ta"},
		{"left right", "a|bc", "i<left>x<right><right>y<esc>", "xab|yc"},
		{"up down", "abc\nd|ef", "i<up>x<down>y<esc>", "axbc\nde|yf"},
		{"alt+x is esc then x", "|a", "iX<alt+l>", "X|a"},
	})
}

func TestListContinuation(t *testing.T) {
	runTable(t, []tcase{
		{"enter bullet", "- a|", "a<enter>b<esc>", "- a\n- |b"},
		{"enter star", "  * a|", "a<enter>b<esc>", "  * a\n  * |b"},
		{"enter numbered", "1. a|", "a<enter>b<esc>", "1. a\n2. |b"},
		{"enter numbered paren", "9) a|", "a<enter>b<esc>", "9) a\n10) |b"},
		{"enter task", "- [x] a|", "a<enter>b<esc>", "- [x] a\n- [ ] |b"},
		{"enter ends empty item", "- a|", "a<enter><enter>b<esc>", "- a\n|b"},
		{"enter before bullet does not continue", "|- a", "i<enter><esc>", "\n|- a"},
		{"o task", "|- [ ] one", "otwo<esc>", "- [ ] one\n- [ ] tw|o"},
		{"o numbered", "|1. one", "otwo<esc>", "1. one\n2. tw|o"},
		{"o bullet", "|- one", "otwo<esc>", "- one\n- tw|o"},
		{"o keeps nested indent", "|  - one", "otwo<esc>", "  - one\n  - tw|o"},
		{"enter splits item text", "- ab|cd", "i<enter>x<esc>", "- ab\n- |xcd"},
		{"enter in-progress task", "- [/] a|", "a<enter>b<esc>", "- [/] a\n- [ ] |b"},
		{"enter cancelled task", "* [-] a|", "a<enter>b<esc>", "* [-] a\n* [ ] |b"},
		{"enter ends empty numbered item", "1. a|", "a<enter><enter>b<esc>", "1. a\n|b"},
		{"enter ends empty task", "- [ ] a|", "a<enter><enter>b<esc>", "- [ ] a\n|b"},
		{"enter ends empty nested item", "  - a|", "a<enter><enter>b<esc>", "  - a\n|b"},
		{"enter ends bare checkbox item", "- [ ]|", "a<enter>b<esc>", "|b"},
	})
}

func TestListContinuationOff(t *testing.T) {
	for _, c := range []tcase{
		{"enter keeps indent only", "  - a|", "a<enter>b<esc>", "  - a\n  |b"},
		{"o keeps indent only", "|1. a", "ob<esc>", "1. a\n|b"},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := load(c.in)
			e.ListContinuation = false
			feed(e, c.keys)
			if g := show(e); g != c.want {
				t.Errorf("%q + %q\n got: %q\nwant: %q", c.in, c.keys, g, c.want)
			}
		})
	}
}

func TestPasteInInsertMode(t *testing.T) {
	e := load("a|b")
	feed(e, "i")
	e.Paste("x\r\ny")
	feed(e, "<esc>")
	if g, w := show(e), "ax\n|yb"; g != w {
		t.Errorf("got %q, want %q", g, w)
	}
	e.Paste("zzz") // ignored outside insert mode
	if g, w := show(e), "ax\n|yb"; g != w {
		t.Errorf("paste in normal mode: got %q, want %q", g, w)
	}
}

func TestModeAndDirty(t *testing.T) {
	e := New("abc")
	if e.Mode != Normal || e.Dirty {
		t.Fatalf("new engine: mode %v dirty %v", e.Mode, e.Dirty)
	}
	feed(e, "i")
	if e.Mode != Insert || e.Mode.String() != "INSERT" {
		t.Fatalf("after i: mode %v", e.Mode)
	}
	feed(e, "x<esc>")
	if e.Mode != Normal || e.Mode.String() != "NORMAL" || !e.Dirty {
		t.Fatalf("after esc: mode %v dirty %v", e.Mode, e.Dirty)
	}
}
