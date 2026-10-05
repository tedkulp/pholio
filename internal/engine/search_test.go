package engine

import (
	"reflect"
	"testing"
)

func TestSearch(t *testing.T) {
	runTable(t, []tcase{
		{"/ forward", "|a foo b foo", "/foo<enter>", "a |foo b foo"},
		{"/ skips match at cursor", "|foo b foo", "/foo<enter>", "foo b |foo"},
		{"/ next line", "|a\nb foo", "/foo<enter>", "a\nb |foo"},
		{"n", "|a foo b foo", "/foo<enter>n", "a foo b |foo"},
		{"n wraps", "|a foo b foo", "/foo<enter>nn", "a |foo b foo"},
		{"N", "|a foo b foo", "/foo<enter>nN", "a |foo b foo"},
		{"N wraps", "|a foo b foo", "/foo<enter>N", "a foo b |foo"},
		{"count n", "|x a x b x c x", "/x<enter>2n", "x a x b x c |x"},
		{"d/ dot repeats pattern", "|a1 x b2 x c3", "d/x<enter>w.", "x |x c3"},
		{"? backward", "foo a foo |b", "?foo<enter>", "foo a |foo b"},
		{"? then n goes back", "foo a foo |b", "?foo<enter>n", "|foo a foo b"},
		{"? then N goes forward", "foo a foo |b", "?foo<enter>nN", "foo a |foo b"},
		{"smartcase lower", "|a FOO", "/foo<enter>", "a |FOO"},
		{"smartcase upper", "|a foo Foo", "/Foo<enter>", "a foo |Foo"},
		{"regex", "|a b12", `/b\d+<enter>`, "a |b12"},
		{"bad regex is literal", "|a b( c", "/b(<enter>", "a |b( c"},
		{"empty reuses last", "|x a x b x", "/x<enter>/<enter>", "x a x b |x"},
		{"not found stays", "a|bc", "/zz<enter>", "a|bc"},
		{"esc cancels", "|a foo", "/foo<esc>", "|a foo"},
		{"backspace edits", "|a fo fx", "/fo<backspace>x<enter>", "a fo |fx"},
		{"backspace on empty exits", "|ab", "/<backspace>l", "a|b"},
		{"d/", "|abc foo", "d/foo<enter>", "|foo"},
		{"d/ undo", "|abc foo", "d/foo<enter>u", "|abc foo"},
		{"dn", "|a foo b foo", "/foo<enter>dn", "a |foo"},
		{"y/ then P", "|ab cd", "y/cd<enter>P", "ab| ab cd"},
		{"v/ extends", "|ab cd ef", "v/cd<enter>d", "|d ef"},
		{"vn extends", "|ab cd ef", "/cd<enter>0vnd", "|d ef"},
		{"grapheme after match", "|é x é", "/é<enter>", "é x |é"},
	})
}

func TestSearchMessages(t *testing.T) {
	e := load("|a foo")
	feed(e, "n")
	if e.Msg != "E35: No previous regular expression" {
		t.Errorf("n with no search: %q", e.Msg)
	}
	feed(e, "/zz<enter>")
	if e.Msg != "E486: Pattern not found: zz" {
		t.Errorf("not found: %q", e.Msg)
	}
	feed(e, "/foo<enter>n")
	if e.Msg != "search hit BOTTOM, continuing at TOP" {
		t.Errorf("wrap: %q", e.Msg)
	}
	feed(e, "N")
	if e.Msg != "search hit TOP, continuing at BOTTOM" {
		t.Errorf("wrap back: %q", e.Msg)
	}
}

func TestCmdLine(t *testing.T) {
	e := load("|abc")
	feed(e, "/ab")
	if p, s := e.CmdLine(); p != "/" || s != "ab" || e.Mode != Search {
		t.Errorf("CmdLine = %q %q mode %v", p, s, e.Mode)
	}
	feed(e, "<esc>?x")
	if p, s := e.CmdLine(); p != "?" || s != "x" {
		t.Errorf("CmdLine = %q %q", p, s)
	}
	feed(e, "<esc>:wq")
	if p, s := e.CmdLine(); p != ":" || s != "wq" || e.Mode != Command || e.Mode.String() != "COMMAND" {
		t.Errorf("CmdLine = %q %q mode %v", p, s, e.Mode)
	}
	feed(e, "<esc>")
	if e.Mode != Normal {
		t.Errorf("mode after esc %v", e.Mode)
	}
}

func TestMatches(t *testing.T) {
	e := load("|foo bar foo\nnone")
	if e.Matches(0) != nil {
		t.Fatal("matches before any search")
	}
	feed(e, "/foo<enter>")
	if got := e.Matches(0); !reflect.DeepEqual(got, [][]int{{0, 3}, {8, 11}}) {
		t.Errorf("Matches(0) = %v", got)
	}
	if got := e.Matches(1); got != nil {
		t.Errorf("Matches(1) = %v", got)
	}
	feed(e, ":noh<enter>")
	if got := e.Matches(0); got != nil {
		t.Errorf("after :noh Matches = %v", got)
	}
	feed(e, "n")
	if got := e.Matches(0); len(got) != 2 {
		t.Errorf("n re-enables highlight, got %v", got)
	}
}

func TestSetSearch(t *testing.T) {
	// SetSearch takes a literal query, as Vault search passes it.
	e := load("|a.b axb A.B")
	e.SetSearch("a.b")
	if got := e.Matches(0); !reflect.DeepEqual(got, [][]int{{0, 3}, {8, 11}}) {
		t.Errorf("Matches = %v", got)
	}
	feed(e, "n")
	if g := show(e); g != "a.b axb |A.B" {
		t.Errorf("n after SetSearch: %q", g)
	}
	feed(e, "N")
	if g := show(e); g != "|a.b axb A.B" {
		t.Errorf("N after SetSearch: %q", g)
	}
}
