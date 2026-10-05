package engine

import "testing"

func TestOperators(t *testing.T) {
	runTable(t, []tcase{
		{"dw", "|foo bar", "dw", "|bar"},
		{"dw last word", "foo |bar\nbaz", "dw", "foo| \nbaz"},
		{"dW", "|a.b c", "dW", "|c"},
		{"db", "foo ba|r", "db", "foo |r"},
		{"de", "|foo bar", "de", "| bar"},
		{"dge", "foo b|ar", "dge", "fo|r"},
		{"d2w = 2dw", "|a b c d", "2d2w", "|"},
		{"dh", "ab|c", "dh", "a|c"},
		{"dl", "a|bc", "dl", "a|c"},
		{"d$", "a|bc", "d$", "|a"},
		{"d0", "ab|c", "d0", "|c"},
		{"d^", "  ab|c", "d^", "  |c"},
		{"dj", "|a\nb\nc", "dj", "|c"},
		{"dk", "a\nb\n|c", "dk", "|a"},
		{"dG", "a\n|b\nc", "dG", "|a"},
		{"dgg", "a\n|b\nc", "dgg", "|c"},
		{"d enter", "|a\nb\nc", "d<enter>", "|c"},
		{"dt", "|foo(bar)", "dt(", "|(bar)"},
		{"df", "|foo(bar)", "df(", "|bar)"},
		{"dF", "foo(ba|r)", "dF(", "foo|r)"},
		{"dT", "foo(ba|r)", "dT(", "foo(|r)"},
		{"d;", "|a,b,c", "f,0d;", "|b,c"},
		{"d}", "|a\nb\n\nc", "d}", "|\nc"},
		{"d{", "a\n\nb\n|c", "d{", "a\n|c"},
		{"d%", "x|(a)y", "d%", "x|y"},
		{"d ctrl+f", "|a\nb\nc", "d<ctrl+f>", "|"},
		{"dd", "a\n|b\nc", "dd", "a\n|c"},
		{"dd last", "a\n|b", "dd", "|a"},
		{"dd only line", "|a", "dd", "|"},
		{"3dd", "|a\nb\nc\nd", "3dd", "|d"},
		{"d invalid motion", "|ab", "dzx", "|b"},
		{"esc cancels op", "|ab", "d<esc>x", "|b"},
		{"cw", "|foo bar", "cwxy<esc>", "x|y bar"},
		{"cw single char", "|a b", "cwz<esc>", "|z b"},
		{"cw on blank", "a| b", "cwx<esc>", "a|xb"},
		{"c2w", "|a b c", "c2wx<esc>", "|x c"},
		{"c$", "a|bc", "c$x<esc>", "a|x"},
		{"cc keeps indent", "  |foo\nbar", "ccx<esc>", "  |x\nbar"},
		{"cj", "  |a\nb\nc", "cjx<esc>", "  |x\nc"},
		{"yw P", "|foo bar", "ywP", "foo| foo bar"},
		{"yy p", "|foo\nbar", "yyp", "foo\n|foo\nbar"},
		{"yy P", "f|oo\nbar", "yyP", "|foo\nfoo\nbar"},
		{"yy p on last line", "a\n|b", "yyp", "a\nb\n|b"},
		{"2yy p", "|a\nb", "2yyjp", "a\nb\n|a\nb"},
		{"y$ cursor stays", "a|bc", "y$", "a|bc"},
		{"yb moves to start", "ab|c", "yb", "|abc"},
		{"yj cursor stays", "|a\nb", "yjGp", "a\nb\n|a\nb"},
	})
}

func TestCommands(t *testing.T) {
	runTable(t, []tcase{
		{"x", "|ab", "x", "|b"},
		{"3x", "|abcd", "3x", "|d"},
		{"x at end", "a|b", "x", "|a"},
		{"delete key", "|ab", "<delete>", "|b"},
		{"X", "ab|c", "X", "a|c"},
		{"s", "|abc", "sx<esc>", "|xbc"},
		{"S", "  a|b\nc", "Sx<esc>", "  |x\nc"},
		{"D", "ab|cd", "D", "a|b"},
		{"C", "ab|cd", "Cx<esc>", "ab|x"},
		{"Y p", "|a\nb", "Yp", "a\n|a\nb"},
		{"r", "|abc", "rx", "|xbc"},
		{"2r", "|abc", "2rx", "x|xc"},
		{"r too many", "|ab", "3rx", "|ab"},
		{"r grapheme", "|👍🏽b", "rx", "|xb"},
		{"J", "|a\n   b", "J", "a| b"},
		{"3J", "|a\nb\nc", "3J", "a b| c"},
		{"J empty", "|a\n\nb", "J", "|a\nb"},
		{"~", "|abc", "2~", "AB|c"},
		{"~ unicode", "|жb", "~", "Ж|b"},
		{"xp", "|ab", "xp", "b|a"},
		{"ddp swaps", "|a\nb", "ddp", "b\n|a"},
		{"P charwise", "a|b", "ylP", "a|bb"},
		{"3p", "|a", "yl3p", "aaa|a"},
		{"p on empty line", "|", "ixy<esc>0d$p", "x|y"},
		{"p multi-line charwise", "|ab\ncd", "y}$p", "ab|ab\ncd\ncd"},
	})
}

func TestRegisters(t *testing.T) {
	runTable(t, []tcase{
		{"named register", "|a\nb", `"ayyjdd"ap`, "a\n|a"},
		{"unnamed after named", "|a\nb", `"ayyjp`, "a\nb\n|a"},
		{"yank register 0 survives delete", "|a\nb", `yyjdd"0p`, "a\n|a"},
		{"empty register", "|a", `"zp`, "|a"},
		{"register with count", "|ab", `"a2yl$"ap`, "aba|b"},
	})
	e := load("|a")
	feed(e, `"zp`)
	if e.Msg != "E353: Nothing in register z" {
		t.Errorf("msg = %q", e.Msg)
	}
}
