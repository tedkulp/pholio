package app_test

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/colorprofile"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"
	"github.com/charmbracelet/x/exp/teatest/v2"

	"github.com/tedkulp/pholio/internal/app"
	"github.com/tedkulp/pholio/internal/palette"
)

func notesPalette() palette.Model {
	return palette.New("Find Note", palette.Type).
		WithPlaceholder("type to filter").
		SetItems([]palette.Item{
			{Text: "Alpha", Detail: "a.md"},
			{Text: "Beta", Detail: "Beta.md"},
			{Text: "Deep", Detail: "Projects/sub/deep.md"},
		})
}

func openPalette(t *testing.T, m app.Model, p palette.Model) (app.Model, *[]palette.Event) {
	t.Helper()
	var evs []palette.Event
	next, _ := m.Update(app.OpenPalette(p, func(ev palette.Event) { evs = append(evs, ev) }))
	return next.(app.Model), &evs
}

func TestPaletteOverTheDimmedPanes(t *testing.T) {
	m, _ := openPalette(t, shell(t, shellVault(), "/vault/a.md"), notesPalette())

	m = typeKeys(m, "e")

	golden.RequireEqual(t, screen(m))
	if c := m.View().Cursor; c == nil || c.X != 10 || c.Y != 2 || c.Shape != tea.CursorBar {
		t.Errorf("cursor = %+v, want a bar after the query", c)
	}
}

func TestPaletteSwallowsKeysAndReportsChoice(t *testing.T) {
	m, evs := openPalette(t, shell(t, shellVault(), "/vault/a.md"), notesPalette())

	m = keys(typeKeys(m, "de"), tea.KeyPressMsg{Code: tea.KeyUp}, enter)

	if got := rows(m)[1]; !strings.Contains(got, "│first note") {
		t.Errorf("second row = %q, want the buffer untouched", got)
	}
	last := (*evs)[len(*evs)-1]
	if last.Kind != palette.Chosen || last.Item.Text != "Deep" || last.Query != "de" {
		t.Errorf("last event = %+v, want Deep chosen with query de", last)
	}
	if strings.Contains(screen(m), "Find Note") {
		t.Error("palette still open after enter")
	}
}

func TestPaletteEdgeOverWideGlyphs(t *testing.T) {
	fsys := shellVault()
	if err := fsys.WriteFile("/vault/a.md", []byte(strings.Repeat("日本語", 20)+"\n")); err != nil {
		t.Fatal(err)
	}
	m := shell(t, fsys, "/vault/a.md")
	for _, w := range []int{80, 81} { // the box's right edge lands mid-glyph at one of these
		m = resize(m, w, 12)
		mo, _ := openPalette(t, m, notesPalette())
		for i, row := range rows(mo) {
			if got := ansi.StringWidth(row); got > w {
				t.Errorf("width %d: row %d is %d cells: %q", w, i, got, row)
			}
		}
	}
}

func TestEscClosesThePalette(t *testing.T) {
	m, evs := openPalette(t, shell(t, shellVault(), "/vault/a.md"), notesPalette())

	m = keys(m, esc)

	if strings.Contains(screen(m), "Find Note") {
		t.Error("palette still open after esc")
	}
	if len(*evs) != 1 || (*evs)[0].Kind != palette.Closed {
		t.Errorf("events = %+v, want one Closed", *evs)
	}
	if m.View().Cursor == nil {
		t.Error("editor cursor not back after closing")
	}
}

func TestSmokePaletteOpensAndCloses(t *testing.T) {
	m := shell(t, shellVault(), "/vault/a.md")
	tm := teatest.NewTestModel(t, m,
		teatest.WithInitialTermSize(smokeW, smokeH),
		teatest.WithProgramOptions(tea.WithColorProfile(colorprofile.Ascii)),
	)
	scr := newVTScreen()

	tm.Send(app.OpenPalette(notesPalette(), func(palette.Event) {}))
	tm.Type("bet")
	scr.waitFor(t, tm, func(s string) bool {
		return strings.Contains(s, "Find Note") && strings.Contains(s, "> bet") && strings.Contains(s, "Beta.md")
	})

	tm.Send(tea.KeyPressMsg{Code: tea.KeyEsc})
	scr.waitFor(t, tm, func(s string) bool {
		return !strings.Contains(s, "Find Note") && strings.Contains(s, "first note")
	})

	tm.Send(ctrlQ)
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}
