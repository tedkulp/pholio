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

func TestDotRepeatVisual(t *testing.T) {
	runTable(t, []tcase{
		{"V> then j.", "|a\nb\nc\nd", "Vj<gt>j.", "\ta\n\t\t|b\n\tc\nd"},
		{"Vd", "|a\nb\nc\nd\ne", "Vjd.", "|e"},
		{"vd one line", "|abcdef", "vld.", "|ef"},
		{"vc replays the insert", "|foo bar baz", "vecX<esc>w.", "X |X baz"},
		{"v~", "|abcdef", "vl~l.", "A|bCdef"},
		{"VJ", "|a\nb\nc\nd", "VjJ.", "a b| c\nd"},
		{"V<", "|\t\ta\n\t\tb", "V<lt>.", "|a\n\t\tb"},
		{"vy is not a change", "|a b c\nx", "dwVjy.", "|c\nx"},
		{"v across lines keeps the end column", "ab|c\ndef\nghi\njkl", "vjhdj.", "abf\ngh|l"},
		{"clamped to the buffer", "|a\nb\nc\nd\ne", "Vjjdj.", "|d"},
		{"clamped to the line", "|abcdef\nxy", "v3ldj.", "ef\n|"},
		{"register reused", "|a\nb\nc", `V"ad."aP`, "|b\nc"},
		{"count ignored", "|a\nb\nc\nd", "Vd3.", "|c\nd"},
		{"undo visual dot", "|a\nb\nc\nd\ne", "Vjd.u", "|c\nd\ne"},
		{"normal change after replaces it", "|abcdef", "vldx.", "|ef"},
	})
}

func TestDotRepeatVisualEndsInNormalMode(t *testing.T) {
	e := load("|abcdef")
	feed(e, "vld.")
	if e.Mode != Normal {
		t.Errorf("mode %v after visual dot", e.Mode)
	}
}
