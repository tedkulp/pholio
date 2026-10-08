package engine

import "testing"

func TestVisualOperators(t *testing.T) {
	runTable(t, []tcase{
		{"vd one char", "a|bc", "vd", "a|c"},
		{"vld", "|abcd", "vld", "|cd"},
		{"vx", "|abcd", "vlx", "|cd"},
		{"v backward", "ab|cd", "vhd", "a|d"},
		{"v across lines", "a|bc\ndef", "vjd", "a|f"},
		{"vy then P", "|abc", "vlyP", "a|babc"},
		{"vy moves to start", "ab|c", "vhy", "a|bc"},
		{"vc", "|abc def", "vecX<esc>", "|X def"},
		{"vs", "|abc", "vlsX<esc>", "|Xc"},
		{"viw", "foo b|ar baz", "viwd", "foo | baz"},
		{"vaw", "foo b|ar baz", "vawd", "foo |baz"},
		{"vo swaps ends", "ab|cd", "vlohd", "|a"},
		{"v~", "|abc", "vl~", "|ABc"},
		{"vJ", "|a\nb\nc", "vjJ", "a| b\nc"},
		{"v count motion", "|abcdef", "v2ld", "|def"},
		{"esc leaves visual", "|abc", "vl<esc>x", "a|c"},
		{"v toggles off", "|abc", "vlvx", "a|c"},
		{"register", "|abc", `v"ayu"aP`, "|aabc"},
		{"invalid key ignored", "|abc", "vzld", "|c"},
		{"Vd", "a\nb|b\nc", "Vd", "a\n|c"},
		{"Vjd", "a\nb|b\nc\nd", "Vjd", "a\n|d"},
		{"Vy p", "|a\nb", "Vyp", "a\n|a\nb"},
		{"Vc", "x\n  a|b\ny", "VcZ<esc>", "x\n  |Z\ny"},
		{"V~", "a|b\ncd", "V~", "|AB\ncd"},
		{"V then v", "a|bc", "Vvd", "a|c"},
		{"v then V", "a|bc\nd", "vVd", "|d"},
		{"vip is linewise", "a\n|b\nc\n\nd", "vipd", "|\nd"},
		{"undo visual delete", "a|bcd", "vldu", "a|bcd"},
	})
}

func TestVisualSelection(t *testing.T) {
	e := load("a|bc\ndef\n\nx")
	if _, _, ok := e.SelectedRange(0); ok {
		t.Fatal("selection outside visual mode")
	}
	feed(e, "vjj")
	type r struct {
		from, to int
		ok       bool
	}
	check := func(want []r) {
		t.Helper()
		for i, w := range want {
			from, to, ok := e.SelectedRange(i)
			if (r{from, to, ok}) != w {
				t.Errorf("%v line %d: %d %d %v, want %+v", e.Mode, i, from, to, ok, w)
			}
		}
	}
	check([]r{{1, 3, true}, {0, 3, true}, {0, 0, true}, {0, 0, false}})
	if e.Mode != Visual || e.Mode.String() != "VISUAL" {
		t.Errorf("mode %v", e.Mode)
	}
	feed(e, "V")
	check([]r{{0, 3, true}, {0, 3, true}, {0, 0, true}, {0, 0, false}})
	if e.Mode.String() != "V-LINE" {
		t.Errorf("mode %v", e.Mode)
	}
}
