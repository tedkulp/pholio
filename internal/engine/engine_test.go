package engine

import (
	"strings"
	"testing"
)

// keys splits "dw<esc>ihi<enter>" into Feed-able key names. <lt> and <gt>
// are the literal < and > keys.
func keys(s string) []string {
	var out []string
	for s != "" {
		if s[0] == '<' {
			if i := strings.IndexByte(s, '>'); i > 0 {
				k := s[1:i]
				if lit, ok := map[string]string{"lt": "<", "gt": ">"}[k]; ok {
					k = lit
				}
				out = append(out, k)
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

// load builds an Engine from text where "|" marks the cursor.
func load(in string) *Engine {
	lines := strings.Split(in, "\n")
	var cur Pos
	for i, l := range lines {
		if j := strings.IndexByte(l, '|'); j >= 0 {
			cur = Pos{i, j}
			lines[i] = l[:j] + l[j+1:]
		}
	}
	e := New(strings.Join(lines, "\n"))
	e.SetCursor(cur)
	return e
}

// show renders the buffer with "|" at the cursor.
func show(e *Engine) string {
	got := strings.Split(strings.TrimSuffix(e.Buf.String(), "\n"), "\n")
	got[e.Cur.Line] = got[e.Cur.Line][:e.Cur.Col] + "|" + got[e.Cur.Line][e.Cur.Col:]
	return strings.Join(got, "\n")
}

func feed(e *Engine, ks string) {
	for _, k := range keys(ks) {
		e.Feed(k)
	}
}

type tcase struct{ name, in, keys, want string }

func runTable(t *testing.T, cases []tcase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := load(c.in)
			feed(e, c.keys)
			if g := show(e); g != c.want {
				t.Errorf("%q + %q\n got: %q\nwant: %q", c.in, c.keys, g, c.want)
			}
		})
	}
}

func TestCharMotions(t *testing.T) {
	runTable(t, []tcase{
		{"l", "|abc", "l", "a|bc"},
		{"2l", "|abc", "2l", "ab|c"},
		{"l stops at last char", "a|bc", "5l", "ab|c"},
		{"right", "|abc", "<right>", "a|bc"},
		{"space", "|abc", " ", "a|bc"},
		{"h", "ab|c", "h", "a|bc"},
		{"h stops at 0", "a|bc", "3h", "|abc"},
		{"left", "ab|c", "<left>", "a|bc"},
		{"backspace", "ab|c", "<backspace>", "a|bc"},
		{"grapheme l", "|é́x", "l", "é́|x"},
		{"emoji l", "|👍🏽x", "l", "👍🏽|x"},
		{"emoji h", "👍🏽|x", "h", "|👍🏽x"},
	})
}
