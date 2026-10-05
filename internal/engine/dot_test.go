package engine

import "testing"

func TestDotRepeat(t *testing.T) {
	runTable(t, []tcase{
		{"dot dw", "|a b c d", "dw..", "|d"},
		{"dot cw", "|a b c", "cwX<esc>w.", "X |X c"},
		{"dot with count", "|a b c d e", "dw2.", "|d e"},
		{"dot keeps original count", "|a b c d e", "2dw.", "|e"},
		{"dot insert", "|x", "ia<esc>.", "|aax"},
		{"dot A", "|a\nb", "A!<esc>j.", "a!\nb|!"},
		{"dot o", "|a", "ob<esc>.", "a\nb\n|b"},
		{"dot x", "|abc", "x.", "|c"},
		{"dot dd", "|a\nb\nc", "dd.", "|c"},
		{"dot r", "|abc", "rxl.", "x|xc"},
		{"dot p", "|a", "ylp..", "aaa|a"},
		{"dot ciw", "|foo bar", "ciwX<esc>w.", "X |X"},
		{"dot di(", "(a) (|b)", "di(0.", "(|) ()"},
		{"dot with register", "|a\nb", `"ayyj"ap.`, "a\nb\na\n|a"},
		{"motion does not reset dot", "|a b c", "xw.", " | c"},
		{"undo dot", "|a b c", "dw.u", "|b c"},
		{"dot before any change", "|ab", ".", "|ab"},
		{"dot list continuation", "|- a", "ob<esc>.", "- a\n- b\n- |b"},
	})
}
