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
	e := load("a|bc\ndef")
	if _, _, _, ok := e.Selection(); ok {
		t.Fatal("selection outside visual mode")
	}
	feed(e, "vj")
	a, z, lw, ok := e.Selection()
	if !ok || lw || a != (Pos{0, 1}) || z != (Pos{1, 2}) {
		t.Errorf("Selection = %v %v %v %v", a, z, lw, ok)
	}
	if e.Mode != Visual || e.Mode.String() != "VISUAL" {
		t.Errorf("mode %v", e.Mode)
	}
	feed(e, "V")
	if _, _, lw, _ := e.Selection(); !lw || e.Mode.String() != "V-LINE" {
		t.Errorf("V-LINE: linewise %v mode %v", lw, e.Mode)
	}
}
