package editor

import (
	"strings"
	"testing"

	"github.com/tedkulp/pholio/internal/engine"
	"github.com/tedkulp/pholio/internal/theme"
)

// kindsOf highlights every line of text through a fence pass.
func kindsOf(text string) (lines []string, ks [][]kind) {
	b := engine.NewBuffer(text)
	f := newFencePass(b, nil)
	for i := range b.LineCount() {
		k, _ := highlight(b.Line(i), f.line(b, i))
		lines, ks = append(lines, b.Line(i)), append(ks, k)
	}
	return lines, ks
}

// kindAt is the kind of the first byte of sub in line i.
func kindAt(t *testing.T, lines []string, ks [][]kind, i int, sub string) kind {
	t.Helper()
	j := strings.Index(lines[i], sub)
	if j < 0 {
		t.Fatalf("%q not in line %d %q", sub, i, lines[i])
	}
	return ks[i][j]
}

func TestGoBlockIsColouredByToken(t *testing.T) {
	lines, ks := kindsOf("```go\nfunc main() {\n\ts := \"hi\" // x\n\tn := 42\n}\n```\n")
	for _, c := range []struct {
		line int
		sub  string
		want kind
	}{
		{0, "```", kMarker},
		{1, "func", kKeyword},
		{1, "main", kFunction},
		{2, `"hi"`, kString},
		{2, "// x", kComment},
		{2, ":=", kOperator},
		{3, "42", kNumber},
		{5, "```", kMarker},
	} {
		if got := kindAt(t, lines, ks, c.line, c.sub); got != c.want {
			t.Errorf("line %d %q: kind %d, want %d", c.line, c.sub, got, c.want)
		}
	}
}

func TestMultiLineCommentColoursEveryLine(t *testing.T) {
	lines, ks := kindsOf("```go\n/* one\ntwo\nthree */\nx := 1\n```\n")
	for i := 1; i <= 3; i++ {
		for j, k := range ks[i] {
			if k != kComment {
				t.Errorf("line %d %q byte %d: kind %d, want comment", i, lines[i], j, k)
			}
		}
	}
	if k := kindAt(t, lines, ks, 4, ":="); k != kOperator {
		t.Errorf("after the comment: kind %d, want operator", k)
	}
}

func TestUnknownOrMissingLanguageIsFlatCode(t *testing.T) {
	for _, open := range []string{"```", "```notalang", "~~~"} {
		_, ks := kindsOf(open + "\nfunc main() { \"hi\" }\n```\n")
		for j, k := range ks[1] {
			if k != kCode {
				t.Errorf("%s: byte %d kind %d, want code", open, j, k)
			}
		}
	}
}

func TestLanguageByNameOrAliasAndInfoStringExtras(t *testing.T) {
	for _, open := range []string{"```py", "```python", "~~~ python", "```py title=\"x\""} {
		lines, ks := kindsOf(open + "\ndef f(): pass\n```\n")
		if k := kindAt(t, lines, ks, 1, "def"); k != kKeyword {
			t.Errorf("%s: def kind %d, want keyword", open, k)
		}
	}
}

func TestUnclosedFenceIsHighlightedToTheEnd(t *testing.T) {
	lines, ks := kindsOf("x\n```go\nfunc a()\n\nfunc b()")
	if k := kindAt(t, lines, ks, 4, "func"); k != kKeyword {
		t.Errorf("kind %d, want keyword", k)
	}
}

func TestCursorMovesDoNotRelexAndEditsOutsideABlockDoNot(t *testing.T) {
	e := engine.New("# t\n\n```go\nfunc a() {}\n```\n\ntext\n")
	m := New(e, "x").SetSize(40, 10)
	th := theme.Default()
	keys := func(ks ...string) {
		for _, k := range ks {
			e.Feed(k)
			_, _ = m.View(th)
		}
	}
	keys()
	_, _ = m.View(th)
	before := lexes
	if before == 0 {
		t.Fatal("block was never lexed")
	}
	keys("j", "j", "j", "k", "G")
	if lexes != before {
		t.Errorf("moving the cursor lexed %d times", lexes-before)
	}
	keys("A", "z", "esc")
	if lexes != before {
		t.Errorf("an edit outside the block lexed %d times", lexes-before)
	}
	keys("g", "g", "3", "j", "A", "z", "esc")
	if lexes == before {
		t.Error("an edit inside the block did not relex it")
	}
}
