package engine

import "testing"

func TestTextObjects(t *testing.T) {
	runTable(t, []tcase{
		{"ciw", "foo b|ar", "ciwX<esc>", "foo |X"},
		{"diw", "foo b|ar baz", "diw", "foo | baz"},
		{"diw on blank", "foo | bar", "diw", "foo|bar"},
		{"diw punct", "a.|..b", "diw", "a|b"},
		{"daw", "foo b|ar baz", "daw", "foo |baz"},
		{"daw last word takes leading space", "foo b|ar", "daw", "fo|o"},
		{"daw on blank", "foo | bar", "daw", "fo|o"},
		{"diW", "x a.|b y", "diW", "x | y"},
		{"daW", "x a.|b y", "daW", "x |y"},
		{"yiw", "foo b|ar", "yiw0P", "ba|rfoo bar"},
		{"dip", "a\n|b\n\nc", "dip", "|\nc"},
		{"dap", "a\n|b\n\nc", "dap", "|c"},
		{"dip on blank", "a\n\n|\nc", "dip", "a\n|c"},
		{`di"`, `say "he|llo" now`, `di"`, `say "|" now`},
		{`ci"`, `say "he|llo" now`, `ci"bye<esc>`, `say "by|e" now`},
		{`da"`, `say "he|llo" now`, `da"`, `say |now`},
		{`di" before quotes`, `|x "a" y`, `di"`, `x "|" y`},
		{`di" escaped`, `"a\"|b"`, `di"`, `"|"`},
		{"di'", "x 'a|b' y", "di'", "x '|' y"},
		{"da'", "x 'a|b' y", "da'", "x |y"},
		{"di`", "x `a|b` y", "di`", "x `|` y"},
		{"da`", "x `a|b` y", "da`", "x |y"},
		{"ci(", "f(a, |b)", "ci(x<esc>", "f(|x)"},
		{"di)", "f(a|b)", "di)", "f(|)"},
		{"dib", "f(a|b)", "dib", "f(|)"},
		{"da(", "f(a|b)x", "da(", "f|x"},
		{"dab", "f(a|b)x", "dab", "f|x"},
		{"da)", "f(a|b)x", "da)", "f|x"},
		{"di( on open paren", "f|(ab)", "di(", "f(|)"},
		{"di( nested", "((a|) b)", "di(", "((|) b)"},
		{"di( outer", "((a) |b)", "di(", "(|)"},
		{"di( multiline", "f(\n  a|,\n)", "di(", "f(|)"},
		{"di( missing", "a|b", "di(", "a|b"},
		{"di[", "x [a |b] y", "di[", "x [|] y"},
		{"di]", "x [a |b] y", "di]", "x [|] y"},
		{"da[", "x [a |b] y", "da[", "x | y"},
		{"da]", "x [a |b] y", "da]", "x | y"},
		{"di{", "{a|b}", "di{", "{|}"},
		{"di}", "{a|b}", "di}", "{|}"},
		{"diB", "{a|b}", "diB", "{|}"},
		{"da{", "x{a|b}", "da{", "|x"},
		{"da}", "x{a|b}", "da}", "|x"},
		{"daB", "x{a|b}", "daB", "|x"},
		{"di<", "<a|b>", "di<", "<|>"},
		{"di>", "<a|b>", "di>", "<|>"},
		{"da<", "x<a|b>", "da<", "|x"},
		{"da>", "x<a|b>", "da>", "|x"},
		{"ci* bold", "a **bo|ld** b", "ci*x<esc>", "a **|x** b"},
		{"di* italic", "a *it|al* b", "di*", "a *|* b"},
		{"da* bold", "a **bo|ld** b", "da*", "a |b"},
		{"da* italic", "a *it|al* b", "da*", "a |b"},
		{"di_", "a _it|al_ b", "di_", "a _|_ b"},
		{"da_", "a _it|al_ b", "da_", "a |b"},
		{"unknown object", "a|b", "dizx", "|a"},
	})
}

func TestCiwOnEmptyLineEntersInsert(t *testing.T) {
	runTable(t, []tcase{
		{"ciw on empty line", "a\n|\nb", "ciwX<esc>", "a\n|X\nb"},
		{"caw on empty line", "a\n|\nb", "cawX<esc>", "a\n|X\nb"},
	})
}
