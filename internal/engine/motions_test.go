package engine

import (
	"strings"
	"testing"
)

func TestLineMotions(t *testing.T) {
	runTable(t, []tcase{
		{"0", "  ab|c", "0", "|  abc"},
		{"home", "  ab|c", "<home>", "|  abc"},
		{"^", "  ab|c", "^", "  |abc"},
		{"$", "|abc", "$", "ab|c"},
		{"2$", "|abc\ndef", "2$", "abc\nde|f"},
		{"end", "|abc", "<end>", "ab|c"},
		{"j", "a|bc\ndef", "j", "abc\nd|ef"},
		{"down", "a|bc\ndef", "<down>", "abc\nd|ef"},
		{"k", "abc\nd|ef", "k", "a|bc\ndef"},
		{"up", "abc\nd|ef", "<up>", "a|bc\ndef"},
		{"2j clamps", "|a\nb", "5j", "a\n|b"},
		{"j keeps column", "abc|d\nx\nabcdef", "jj", "abcd\nx\nabc|def"},
		{"$ then j sticks to end", "|ab\nabcdef", "$j", "ab\nabcde|f"},
		{"j keeps cells over wide chars", "ab|c\n日本語", "j", "abc\n日|本語"},
		{"j keeps cells over tab", "abcd|e\n\txy", "j", "abcde\n\t|xy"},
		{"enter", "|a\n  b", "<enter>", "a\n  |b"},
		{"gg", "a\n|b\nc", "gg", "|a\nb\nc"},
		{"gg to first non-blank", "  a\n|b", "gg", "  |a\nb"},
		{"2gg", "a\nb\n|c", "2gg", "a\n|b\nc"},
		{"G", "|a\nb\n c", "G", "a\nb\n |c"},
		{"1G", "a\nb\n|c", "1G", "|a\nb\nc"},
	})
}

func TestPageMotions(t *testing.T) {
	text := func(cur int) string {
		var ls []string
		for i := range 30 {
			l := "x"
			if i == cur {
				l = "|x"
			}
			ls = append(ls, l)
		}
		return strings.Join(ls, "\n")
	}
	cases := []struct {
		keys string
		from int
		want int
	}{
		{"<ctrl+f>", 0, 8},
		{"<pgdown>", 0, 8},
		{"2<ctrl+f>", 0, 16},
		{"<ctrl+b>", 20, 12},
		{"<pgup>", 20, 12},
		{"<ctrl+b>", 3, 0},
		{"<ctrl+d>", 0, 5},
		{"<ctrl+u>", 20, 15},
		{"<ctrl+f>", 27, 29},
	}
	for _, c := range cases {
		e := load(text(c.from))
		e.PageLines = 10
		feed(e, c.keys)
		if e.Cur.Line != c.want {
			t.Errorf("line %d + %q: got line %d, want %d", c.from, c.keys, e.Cur.Line, c.want)
		}
	}
}
