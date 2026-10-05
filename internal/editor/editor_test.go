package editor_test

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"

	"github.com/tedkulp/pholio/internal/editor"
	"github.com/tedkulp/pholio/internal/engine"
	"github.com/tedkulp/pholio/internal/theme"
)

var th = theme.Default()

// open puts text in an editor pane of w×h cells (status line not included).
func open(text string, w, h int) editor.Model {
	return editor.New(engine.New(text), "note.md").SetSize(w, h)
}

// key builds the key press a terminal sends for k: a single character, or a
// name such as "esc", "enter" or "ctrl+d".
func key(k string) tea.KeyPressMsg {
	if utf8.RuneCountInString(k) == 1 {
		r, _ := utf8.DecodeRuneInString(k)
		return tea.KeyPressMsg{Code: r, Text: k}
	}
	names := map[string]tea.KeyPressMsg{
		"esc":       {Code: tea.KeyEsc},
		"enter":     {Code: tea.KeyEnter},
		"backspace": {Code: tea.KeyBackspace},
		"ctrl+d":    {Code: 'd', Mod: tea.ModCtrl},
		"ctrl+f":    {Code: 'f', Mod: tea.ModCtrl},
		"alt+x":     {Code: 'x', Mod: tea.ModAlt},
	}
	msg, ok := names[k]
	if !ok {
		panic("unknown key " + k)
	}
	return msg
}

// feed types keys: plain characters, with <name> for named keys ("dd<esc>").
func feed(m editor.Model, keys string) editor.Model {
	for keys != "" {
		k := keys[:1]
		if r, n := utf8.DecodeRuneInString(keys); n > 1 {
			k = string(r)
		}
		if keys[0] == '<' {
			if end := strings.IndexByte(keys, '>'); end > 0 {
				k = keys[1:end]
				keys = keys[end+1:]
				m = m.Update(key(k))
				continue
			}
		}
		keys = keys[len(k):]
		m = m.Update(key(k))
	}
	return m
}

// screen is the pane as plain text with the status line below it and the
// cursor marked by a row "cursor x,y shape".
func screen(m editor.Model, w int) string {
	text, cur := m.View(th)
	out := ansi.Strip(text) + "\n" + ansi.Strip(m.StatusLine(th, w)) + "\n"
	if cur == nil {
		return out + "cursor hidden\n"
	}
	shape := map[tea.CursorShape]string{tea.CursorBlock: "block", tea.CursorBar: "bar", tea.CursorUnderline: "underline"}[cur.Shape]
	return out + fmt.Sprintf("cursor %d,%d %s\n", cur.X, cur.Y, shape)
}

const prose = "# Wrapping\n" +
	"The quick brown fox jumps over the lazy dog and keeps running.\n" +
	"short\n" +
	"Averyveryverylongwordthatcannotbreak here.\n"

func TestWrapBreaksAtWordsWithinThePane(t *testing.T) {
	m := open(prose, 20, 10)

	golden.RequireEqual(t, screen(m, 20))
}

func TestNowrapScrollsSidewaysToFollowTheCursor(t *testing.T) {
	m := open(prose, 20, 6).SetWrap(false)

	m = feed(m, "j$")

	golden.RequireEqual(t, screen(m, 20))
}

func TestNowrapScrollsBackWhenTheCursorReturnsLeft(t *testing.T) {
	m := open(prose, 20, 6).SetWrap(false)

	m = feed(m, "j$0")

	if got := screen(m, 20); !strings.HasPrefix(got, "# Wrapping") {
		t.Fatalf("view did not scroll back to column 0:\n%s", got)
	}
}

// numbered is a Note of n lines "line 1" … "line n".
func numbered(n int) string {
	var sb strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&sb, "line %d\n", i)
	}
	return sb.String()
}

func TestScrolloffKeepsThreeLinesBelowTheCursor(t *testing.T) {
	m := open(numbered(30), 20, 8)

	m = feed(m, "4j") // line 5: rows 1-8 still show 3 below it

	golden.RequireEqual(t, screen(m, 20))
}

func TestScrolloffScrollsDownOneLineAtATime(t *testing.T) {
	m := open(numbered(30), 20, 8)

	m = feed(m, "5j") // line 6 needs line 9 visible

	golden.RequireEqual(t, screen(m, 20))
}

func TestJumpToEndShowsTheLastScreenful(t *testing.T) {
	m := open(numbered(30), 20, 8)

	m = feed(m, "G")

	golden.RequireEqual(t, screen(m, 20))
}

func TestScrolloffKeepsThreeLinesAboveTheCursor(t *testing.T) {
	m := open(numbered(30), 20, 8)

	m = feed(m, "G4k") // line 26 is 3 below the top: no scroll yet
	m = feed(m, "k")   // line 25 pulls the view up one line

	golden.RequireEqual(t, screen(m, 20))
}

func TestScrolloffCountsWrappedRows(t *testing.T) {
	long := "a long line that wraps over several rows of the pane"
	m := open(numbered(5)+long+"\n"+numbered(5), 20, 8)

	m = feed(m, "5j") // the long line's first row: 3 rows below it, 2 of them its own

	golden.RequireEqual(t, screen(m, 20))
}

func TestCursorDeepInAParagraphTallerThanThePaneStaysVisible(t *testing.T) {
	para := strings.Repeat("word ", 40) // 10 rows at width 20
	m := open(para, 20, 4)

	m = feed(m, "$")

	golden.RequireEqual(t, screen(m, 20))
}

func TestStatusLineInInsertModeShowsModifiedAndABarCursor(t *testing.T) {
	m := open("hello world\n", 30, 2)

	m = feed(m, "wiyou ")

	golden.RequireEqual(t, screen(m, 30))
}

func TestStatusLineShowsPendingKeysAndAnUnderlineCursor(t *testing.T) {
	m := open("hello world\n", 30, 2)

	m = feed(m, `"a2d`)

	golden.RequireEqual(t, screen(m, 30))
}

func TestStatusLineKeepsLineAndColumnWhenNarrow(t *testing.T) {
	m := open("hello\n", 12, 1)

	got := ansi.Strip(m.StatusLine(th, 12))

	if got != " NORMAL 1:1 " {
		t.Fatalf("status line = %q", got)
	}
}

func TestStatusLineColumnCountsCells(t *testing.T) {
	m := open("日本\tx\n", 30, 1)

	m = feed(m, "$")

	if got := ansi.Strip(m.StatusLine(th, 30)); !strings.HasSuffix(got, " 1:9 ") {
		t.Fatalf("status line = %q, want column 9 (after 2 wide glyphs and a tab)", got)
	}
}

func TestStatusLineDrawsThroughThemeSlots(t *testing.T) {
	m := feed(open("hi\n", 30, 1), "x")
	line := m.StatusLine(th, 30)

	for slot, text := range map[theme.Slot]string{
		theme.UIModeNormal:  " NORMAL ",
		theme.UIStatusFile:  " note.md",
		theme.UIStatusDirty: " [+]",
	} {
		if !strings.Contains(line, th.Style(slot).Render(text)) {
			t.Errorf("%q not drawn with %s in %q", text, slot, line)
		}
	}
	if m = feed(m, "i"); !strings.Contains(m.StatusLine(th, 30), th.Style(theme.UIModeInsert).Render(" INSERT ")) {
		t.Error("insert mode not drawn with ui.mode_insert")
	}
}

func TestAltKeyOutsideNormalModeIsEscThenKey(t *testing.T) {
	m := open("abc\n", 20, 2)

	m = feed(m, "A!<alt+x>") // a fast <esc>x arrives as alt+x

	if got := m.Engine().Buf.Line(0); got != "abc" {
		t.Fatalf("line = %q, want the ! deleted by x after esc", got)
	}
	if m.Engine().Mode != engine.Normal {
		t.Fatalf("mode = %v, want normal", m.Engine().Mode)
	}
}

func TestInsertCursorPastAFullRowGetsItsOwnRow(t *testing.T) {
	m := open("abcdefghij\nnext\n", 10, 4)

	m = feed(m, "A")

	golden.RequireEqual(t, screen(m, 10))
}

func TestCursorSitsAfterWideGlyphsAndTabs(t *testing.T) {
	m := open("日本\tx\n", 20, 1)

	m = feed(m, "$")

	if _, c := m.View(th); c == nil || c.X != 8 || c.Y != 0 {
		t.Fatalf("cursor = %+v, want 8,0", c)
	}
}

func TestNowrapCutsAWideGlyphAtTheLeftEdge(t *testing.T) {
	m := open("ab日本語xy\n", 5, 1).SetWrap(false)

	m = feed(m, "$") // y at cell 9 scrolls the view to start at cell 5, inside 本

	text, c := m.View(th)
	if got := ansi.Strip(text); got != " 語xy" {
		t.Fatalf("row = %q", got)
	}
	if c.X != 4 {
		t.Fatalf("cursor x = %d, want 4", c.X)
	}
}

func TestPasteInsertsInInsertMode(t *testing.T) {
	m := open("\n", 20, 2)

	m = m.Update(key("i")).Update(tea.PasteMsg{Content: "pasted"})

	if got := m.Engine().Buf.Line(0); got != "pasted" {
		t.Fatalf("line = %q", got)
	}
}

func TestPageLinesFollowThePaneHeight(t *testing.T) {
	m := open(numbered(100), 20, 10)

	if m.Engine().PageLines != 10 {
		t.Fatalf("PageLines = %d, want 10", m.Engine().PageLines)
	}
}
