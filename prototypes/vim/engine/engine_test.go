// PROTOTYPE — demonstrates the engine is testable with no Bubble Tea at all:
// text in, keys in, text + cursor out.

package engine

import (
	"strings"
	"testing"
)

type memFS map[string]string

func (m memFS) ReadFile(p string) (string, error) { return m[p], nil }
func (m memFS) WriteFile(p, d string) error       { m[p] = d; return nil }

// keys splits "dw<esc>ihi<enter>" into Feed-able key names.
func keys(s string) []string {
	var out []string
	for s != "" {
		if s[0] == '<' {
			if i := strings.IndexByte(s, '>'); i > 0 {
				out = append(out, s[1:i])
				s = s[i+1:]
				continue
			}
		}
		r := []rune(s)[0]
		out = append(out, string(r))
		s = s[len(string(r)):]
	}
	return out
}

// "|" in the input marks the cursor; in want it marks where it must end up.
func run(t *testing.T, in, ks, want string) {
	t.Helper()
	lines := strings.Split(in, "\n")
	var cur Pos
	for i, l := range lines {
		if j := strings.IndexByte(l, '|'); j >= 0 {
			cur = Pos{i, j}
			lines[i] = l[:j] + l[j+1:]
		}
	}
	e := New(memFS{"f": strings.Join(lines, "\n")}, "f")
	e.Cur = cur
	e.want = Cells(e.Buf.Line(cur.Line), cur.Col)
	for _, k := range keys(ks) {
		e.Feed(k)
	}
	got := strings.Split(strings.TrimSuffix(e.Buf.String(), "\n"), "\n")
	got[e.Cur.Line] = got[e.Cur.Line][:e.Cur.Col] + "|" + got[e.Cur.Line][e.Cur.Col:]
	if g := strings.Join(got, "\n"); g != want {
		t.Errorf("%q + %q\n got: %q\nwant: %q", in, ks, g, want)
	}
}

func TestEngine(t *testing.T) {
	cases := []struct{ name, in, keys, want string }{
		{"w", "|foo bar.baz", "w", "foo |bar.baz"},
		{"3w", "|foo bar.baz", "3w", "foo bar.|baz"},
		{"W", "|foo bar.baz qux", "2W", "foo bar.baz |qux"},
		{"b", "foo bar|", "b", "foo |bar"},
		{"e", "|foo bar", "e", "fo|o bar"},
		{"ge", "foo bar|", "ge", "fo|o bar"},
		{"w across lines", "|foo\n\nbar", "w", "foo\n|\nbar"},
		{"dw", "|foo bar", "dw", "|bar"},
		{"dw last word", "foo |bar\nbaz", "dw", "foo| \nbaz"},
		{"cw", "|foo bar", "cwxy<esc>", "x|y bar"},
		{"cw single char", "|a b", "cwz<esc>", "|z b"},
		{"d2w = 2dw", "|a b c d", "2d2w", "|"},
		{"dd", "a\n|b\nc", "dd", "a\n|c"},
		{"dd last", "a\n|b", "dd", "|a"},
		{"3dd", "|a\nb\nc\nd", "3dd", "|d"},
		{"cc keeps indent", "  |foo\nbar", "ccx<esc>", "  |x\nbar"},
		{"yyp", "|foo\nbar", "yyp", "foo\n|foo\nbar"},
		{"ddp swaps", "|a\nb", "ddp", "b\n|a"},
		{"x p", "|ab", "xp", "b|a"},
		{"3x", "|abcd", "3x", "|d"},
		{"D", "ab|cd", "D", "a|b"},
		{"f", "|a,b,c", "2f,", "a,b|,c"},
		{"t ;", "|a,b,c", "t,;", "a,|b,c"},
		{"dt", "|foo(bar)", "dt(", "|(bar)"},
		{"$", "|abc", "$", "ab|c"},
		{"diw", "foo b|ar baz", "diw", "foo | baz"},
		{"daw", "foo b|ar baz", "daw", "foo |baz"},
		{"ci(", "f(a, |b)", "ci(x<esc>", "f(|x)"},
		{"da[", "x [a |b] y", "da[", "x | y"},
		{"ci\"", `say "he|llo" now`, `ci"bye<esc>`, `say "by|e" now`},
		{"di( multiline", "f(\n  a|,\n)", "di(", "f(|)"},
		{"ci* bold", "a **bo|ld** b", "ci*x<esc>", "a **|x** b"},
		{"dap", "a\n|b\n\nc", "dap", "|c"},
		{"}", "|a\nb\n\nc", "}", "a\nb\n|\nc"},
		{"d}", "|a\nb\n\nc", "d}", "|\nc"},
		{"%", "|(a [b] c)", "%", "(a [b] c|)"},
		{"gg G", "a\n|b\nc", "Ggg", "|a\nb\nc"},
		{"5G", "a\nb\n|c", "1G", "|a\nb\nc"},
		{"j keeps column", "abc|d\nx\nabcdef", "jj", "abcd\nx\nabc|def"},
		{"$ then j sticks to end", "|ab\nabcdef", "$j", "ab\nabcde|f"},
		{"undo", "|foo bar", "dwu", "|foo bar"},
		{"undo insert session", "|x", "ihello<esc>u", "|x"},
		{"redo", "|foo bar", "dwu<ctrl+r>", "|bar"},
		{"dot dw", "|a b c d", "dw..", "|d"},
		{"dot cw", "|a b c", "cwX<esc>w.", "X |X c"},
		{"dot with count", "|a b c d e", "dw2.", "|d e"},
		{"dot insert", "|x", "ia<esc>.", "|aax"},
		{"undo dot", "|a b c", "dw.u", "|b c"},
		{"o list continuation", "|- [ ] one", "otwo<esc>", "- [ ] one\n- [ ] tw|o"},
		{"enter numbered", "1. a|", "a<enter>b<esc>", "1. a\n2. |b"},
		{"enter ends empty item", "- a|", "a<enter><enter>b<esc>", "- a\n|b"},
		{"visual d", "a|bcd", "vld", "a|d"},
		{"visual line y P", "|a\nb", "VjyP", "|a\nb\na\nb"},
		{"visual iw c", "foo b|ar", "viwcX<esc>", "foo |X"},
		{"visual o", "a|bcd", "vlohd", "|d"},
		{"J", "|a\n   b", "J", "a| b"},
		{"r", "|abc", "2rx", "x|xc"},
		{"~", "|abc", "2~", "AB|c"},
		{"grapheme x", "|👍🏽x", "x", "|x"},
		{"grapheme l", "|é́x", "l", "é́|x"},
		{"f matches grapheme base", "|a 👍🏽 b", "f👍", "a |👍🏽 b"},
		{"cjk f", "|日本語", "f語", "日本|語"},
		{"search", "|foo bar foo", "/foo<enter>", "foo bar |foo"},
		{"search wrap n", "foo |bar foo", "/foo<enter>n", "|foo bar foo"},
		{"d/", "|foo bar baz", "d/baz<enter>", "|baz"},
		{"register", "|a\nb", `"ayyjdd"ap`, "a\n|a"},
		{":3", "|a\nb\nc", ":3<enter>", "a\nb\n|c"},
		{"esc cancels op", "|ab", "d<esc>x", "|b"},
		{"alt+x is esc then x", "|a", "iX<alt+u>", "|a"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { run(t, c.in, c.keys, c.want) })
	}
}

func TestWriteQuit(t *testing.T) {
	fs := memFS{"f": "a\n"}
	e := New(fs, "f")
	for _, k := range keys("xib<esc>:q<enter>") {
		e.Feed(k)
	}
	if e.Quit || !strings.HasPrefix(e.Msg, "E37") {
		t.Fatalf(":q on dirty buffer should refuse, msg=%q", e.Msg)
	}
	for _, k := range keys(":wq<enter>") {
		e.Feed(k)
	}
	if !e.Quit || fs["f"] != "b\n" {
		t.Fatalf("quit=%v file=%q", e.Quit, fs["f"])
	}
}
