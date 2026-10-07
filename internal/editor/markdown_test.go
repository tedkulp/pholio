package editor_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"

	"github.com/tedkulp/pholio/internal/editor"
	"github.com/tedkulp/pholio/internal/theme"
)

const marked = "# Notes\n" +
	"See [[Other note]] and **bold** text.\n" +
	"Run `go test` or read [the docs](https://example.com).\n" +
	"- [ ] call **Bob** [due:: 2026-10-10] #work\n"

func TestConcealHidesMarkupOffTheCursorLine(t *testing.T) {
	m := open(marked, 40, 6)

	golden.RequireEqual(t, screen(m, 40))
}

func TestConcealOffShowsAllMarkup(t *testing.T) {
	m := open(marked, 40, 6).SetConceal(false)

	golden.RequireEqual(t, screen(m, 40))
}

func TestCursorLineRevealsItsMarkup(t *testing.T) {
	m := open(marked, 40, 6)

	m = feed(m, "jj")

	golden.RequireEqual(t, screen(m, 40))
}

func TestFencedCodeIsNeitherConcealedNorStyledAsMarkdown(t *testing.T) {
	m := open("x\n```\n**not bold** [[x]]\n```\nafter **bold**\n", 30, 6)

	view, _ := m.View(th)

	rows := plainRows(view)
	if rows[2] != "**not bold** [[x]]" {
		t.Errorf("fenced row = %q, want it shown as written", rows[2])
	}
	if rows[4] != "after bold" {
		t.Errorf("row after the fence = %q, want markdown again", rows[4])
	}
	if !strings.Contains(view, styled(theme.MarkdownCode, "**not bold** [[x]]")) {
		t.Errorf("fenced line not drawn as code:\n%q", view)
	}
}

func TestHighlightingDrawsThroughMarkdownSlots(t *testing.T) {
	text := "# Title here\n" +
		"plain **bold** and *it* with `code` and [[Link]] #tag\n" +
		"> quoted\n" +
		"- [ ] open task [due:: 2026-10-10] 📅 2026-10-11 due:2026-10-12\n" +
		"- [x] done task\n" +
		"- [-] cancelled task\n" +
		"1. numbered\n"
	m := feed(open(text, 80, 10), "G")

	view, _ := m.View(th)

	for _, c := range []struct {
		slot theme.Slot
		text string
	}{
		{theme.MarkdownHeading, " Title here"},
		{theme.MarkdownMarker, "#"},
		{theme.MarkdownBold, "bold"},
		{theme.MarkdownItalic, "it"},
		{theme.MarkdownCode, "code"},
		{theme.MarkdownLink, "Link"},
		{theme.MarkdownTag, "#tag"},
		{theme.MarkdownQuote, " quoted"},
		{theme.MarkdownBullet, "-"},
		{theme.MarkdownTaskBox, "[ ]"},
		{theme.MarkdownMeta, "[due:: 2026-10-10]"},
		{theme.MarkdownMeta, "📅 2026-10-11"},
		{theme.MarkdownTaskDone, "[x] done task"},
		{theme.MarkdownTaskDone, "[-] cancelled task"},
		{theme.MarkdownBullet, "1."},
		{theme.UIBase, "plain "},
	} {
		if !strings.Contains(view, styled(c.slot, c.text)) {
			t.Errorf("%q is not drawn with %s", c.text, c.slot)
		}
	}
	if strings.Contains(view, styled(theme.MarkdownMeta, "due:2026-10-12")) {
		t.Error("key:value text is drawn as Task Metadata")
	}
}

// styled is text as the pane draws it through slot, over the base text
// style.
func styled(slot theme.Slot, text string) string {
	return th.Style(slot).Inherit(th.Style(theme.UIBase)).Render(text)
}

// plainRows is the view without styling, one entry per row, trailing
// padding removed.
func plainRows(view string) []string {
	rows := strings.Split(ansi.Strip(view), "\n")
	for i, r := range rows {
		rows[i] = strings.TrimRight(r, " ")
	}
	return rows
}

func TestStrikethroughConcealsItsMarkersOffTheCursorLine(t *testing.T) {
	m := open("x\na ~~b~~ c\n", 30, 4)

	view, _ := m.View(th)

	if rows := plainRows(view); rows[1] != "a b c" {
		t.Errorf("row = %q, want the ~~ hidden", rows[1])
	}
	if !strings.Contains(view, styled(theme.MarkdownStrike, "b")) {
		t.Errorf("b is not drawn with %s:\n%q", theme.MarkdownStrike, view)
	}
}

func TestStrikethroughMarkersShowOnTheCursorLineAndWithConcealOff(t *testing.T) {
	for name, m := range map[string]editor.Model{
		"cursor line": feed(open("x\na ~~b~~ c\n", 30, 4), "j"),
		"conceal off": open("x\na ~~b~~ c\n", 30, 4).SetConceal(false),
	} {
		view, _ := m.View(th)

		if rows := plainRows(view); rows[1] != "a ~~b~~ c" {
			t.Errorf("%s: row = %q, want the ~~ shown", name, rows[1])
		}
		if !strings.Contains(view, styled(theme.MarkdownMarker, "~~")) {
			t.Errorf("%s: ~~ is not drawn as a marker:\n%q", name, view)
		}
		if !strings.Contains(view, styled(theme.MarkdownStrike, "b")) {
			t.Errorf("%s: b is not drawn with %s", name, theme.MarkdownStrike)
		}
	}
}

func TestTildesThatAreNotStrikethroughStayPlain(t *testing.T) {
	for _, c := range []struct {
		line string
		slot theme.Slot
		text string
	}{
		{"a ~b~ c", theme.UIBase, "a ~b~ c"},
		{"a ~~~~ b", theme.UIBase, "a ~~~~ b"},
		{"~~~", theme.MarkdownMarker, "~~~"}, // a bare ~~~ line is a fence
		{"a `~~b~~` c", theme.MarkdownCode, "~~b~~"},
		{"- [x] done ~~x~~", theme.MarkdownTaskDone, "[x] done ~~x~~"},
	} {
		m := open("x\n"+c.line+"\n", 30, 4)

		view, _ := m.View(th)

		if row := plainRows(view)[1]; !strings.Contains(row, c.text) {
			t.Errorf("%q: row = %q, want %q shown", c.line, row, c.text)
		}
		if !strings.Contains(view, styled(c.slot, c.text)) {
			t.Errorf("%q: %q is not drawn with %s", c.line, c.text, c.slot)
		}
	}
}

func TestTildeFenceIsUnchanged(t *testing.T) {
	m := open("x\n~~~\n~~not struck~~\n~~~\n", 30, 6)

	view, _ := m.View(th)

	if rows := plainRows(view); rows[2] != "~~not struck~~" {
		t.Errorf("fenced row = %q, want it shown as written", rows[2])
	}
	if !strings.Contains(view, styled(theme.MarkdownCode, "~~not struck~~")) {
		t.Errorf("fenced line not drawn as code:\n%q", view)
	}
}

func TestBoldInsideStrikethroughIsBold(t *testing.T) {
	m := open("x\n~~**y**~~\n", 30, 4)

	view, _ := m.View(th)

	if rows := plainRows(view); rows[1] != "y" {
		t.Errorf("row = %q, want every marker hidden", rows[1])
	}
	if !strings.Contains(view, styled(theme.MarkdownBold, "y")) {
		t.Errorf("y is not drawn bold:\n%q", view)
	}
}
