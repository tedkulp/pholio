package form_test

import (
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/tedkulp/pholio/internal/form"
	"github.com/tedkulp/pholio/internal/theme"
)

func key(s string) tea.KeyPressMsg {
	switch s {
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEsc}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "shift+tab":
		return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
	case "home":
		return tea.KeyPressMsg{Code: tea.KeyHome}
	case "end":
		return tea.KeyPressMsg{Code: tea.KeyEnd}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "delete":
		return tea.KeyPressMsg{Code: tea.KeyDelete}
	case "ctrl+u":
		return tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl}
	case "space":
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	}
	r := []rune(s)[0]
	return tea.KeyPressMsg{Code: r, Text: s}
}

func feed(f form.Model, ks ...string) (form.Model, form.Event) {
	var ev form.Event
	for _, k := range ks {
		f, ev = f.Update(key(k))
	}
	return f, ev
}

// typeText sends each rune of s as a key.
func typeText(f form.Model, s string) form.Model {
	for _, r := range s {
		f, _ = f.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	return f
}

func sample() form.Model {
	return form.New("Task",
		form.Field{Label: "Description", Value: "Pay rent"},
		form.Field{Label: "Status", Kind: form.Choice, Choices: []string{"open", "done", "cancelled"}, Value: "open"},
		form.Field{Label: "Due", Placeholder: "date"},
	)
}

func TestTypingEditsTheFocusedTextRowAtItsCursor(t *testing.T) {
	f := typeText(sample(), " now")
	f, _ = feed(f, "home", "right", "right", "right", "delete")
	f = typeText(f, "!")
	f, _ = feed(f, "end", "backspace", "space")
	if got := f.Values()[0]; got != "Pay!rent no " {
		t.Errorf("description = %q", got)
	}
	f, _ = feed(f, "ctrl+u")
	if got := f.Values()[0]; got != "" {
		t.Errorf("after ctrl+u = %q", got)
	}
}

func TestTabAndShiftTabMoveBetweenRowsAndWrap(t *testing.T) {
	f, _ := feed(sample(), "tab", "tab")
	f = typeText(f, "fri")
	f, _ = feed(f, "tab", "shift+tab", "shift+tab")
	if f.Focus() != 1 {
		t.Errorf("focus = %d, want 1", f.Focus())
	}
	if got := f.Values(); !reflect.DeepEqual(got, []string{"Pay rent", "open", "fri"}) {
		t.Errorf("values = %q", got)
	}
}

func TestSpaceAndArrowsCycleAChoice(t *testing.T) {
	f, _ := feed(sample(), "tab", "space")
	if got := f.Values()[1]; got != "done" {
		t.Errorf("after space = %q", got)
	}
	f, _ = feed(f, "right", "right")
	if got := f.Values()[1]; got != "open" {
		t.Errorf("after → → = %q (should wrap)", got)
	}
	f, _ = feed(f, "left")
	if got := f.Values()[1]; got != "cancelled" {
		t.Errorf("after ← = %q", got)
	}
	f = typeText(f, "x")
	if got := f.Values()[1]; got != "cancelled" {
		t.Errorf("typing changed a choice: %q", got)
	}
}

func TestEnterSavesAndEscCloses(t *testing.T) {
	_, ev := feed(sample(), "tab", "space", "enter")
	if ev.Kind != form.Saved || !reflect.DeepEqual(ev.Values, []string{"Pay rent", "done", ""}) {
		t.Errorf("enter = %+v", ev)
	}
	_, ev = feed(sample(), "x", "esc")
	if ev.Kind != form.Closed {
		t.Errorf("esc = %+v", ev)
	}
}

func TestPasteGoesIntoATextRowOnOneLine(t *testing.T) {
	f, _ := sample().Paste(" a\nb")
	if got := f.Values()[0]; got != "Pay rent a b" {
		t.Errorf("description = %q", got)
	}
	f, _ = feed(f, "tab")
	f, _ = f.Paste("zzz")
	if got := f.Values()[1]; got != "open" {
		t.Errorf("paste changed a choice: %q", got)
	}
}

func TestViewDrawsOneRowPerFieldWithTheCursor(t *testing.T) {
	f := sample().WithHint("enter save · esc close")
	box, x, y, cursor := f.View(theme.Default(), 100, 30)
	plain := ansi.Strip(box)
	for _, want := range []string{"╭─ Task ", "Description  Pay rent", "Status       ‹ open ›", "Due          date", "enter save · esc close"} {
		if !strings.Contains(plain, want) {
			t.Errorf("view lacks %q:\n%s", want, plain)
		}
	}
	if x != 10 || y != 1 {
		t.Errorf("box at %d,%d, want the palette's 10,1", x, y)
	}
	// "│ " + label column + "Pay rent"
	if cursor == nil || cursor.X != x+2+13+8 || cursor.Y != y+1 {
		t.Errorf("cursor = %+v", cursor)
	}
	f, _ = feed(f, "tab")
	if _, _, _, cursor := f.View(theme.Default(), 100, 30); cursor != nil {
		t.Errorf("a choice row shows a cursor: %+v", cursor)
	}
}

func TestInvalidRowsAreDrawnInTheErrorStyleUntilEdited(t *testing.T) {
	th := theme.Default()
	f := typeText(sample(), "")
	f, _ = feed(f, "tab", "tab")
	f = typeText(f, "someday").WithInvalid(2)
	if !f.Invalid(2) || f.Invalid(0) {
		t.Fatal("WithInvalid did not mark only row 2")
	}
	box, _, _, _ := f.View(th, 100, 30)
	if !strings.Contains(box, th.Style(theme.UIError).Render("someday")) {
		t.Errorf("invalid row not in ui.error:\n%q", box)
	}
	f = typeText(f, "x")
	if f.Invalid(2) {
		t.Error("editing the row kept its mark")
	}
}
