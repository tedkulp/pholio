package app_test

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/tedkulp/pholio/internal/app"
	"github.com/tedkulp/pholio/internal/engine"
	"github.com/tedkulp/pholio/internal/palette"
)

func TestMouseModeIsRequestedByDefault(t *testing.T) {
	m := shell(t, shellVault(), "/vault/a.md")

	if got := m.View().MouseMode; got != tea.MouseModeCellMotion {
		t.Errorf("MouseMode = %v, want cell motion", got)
	}
}

func TestMouseFalseRequestsNoMouseMode(t *testing.T) {
	fsys := shellVault()
	if err := fsys.WriteFile(userConfig, []byte("mouse = false\n")); err != nil {
		t.Fatal(err)
	}
	m := shell(t, fsys, "/vault/a.md")

	if got := m.View().MouseMode; got != tea.MouseModeNone {
		t.Errorf("MouseMode = %v, want none", got)
	}
}

func send(m app.Model, msg tea.Msg) app.Model {
	next, _ := m.Update(msg)
	return next.(app.Model)
}

func click(x, y int) tea.MouseClickMsg {
	return tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft}
}

func release(x, y int) tea.MouseReleaseMsg {
	return tea.MouseReleaseMsg{X: x, Y: y, Button: tea.MouseLeft}
}

// sidebarFocused reports whether the sidebar has focus: the editor's
// cursor is hidden then.
func sidebarFocused(m app.Model) bool { return m.View().Cursor == nil }

func TestClickOnAFolderTogglesItAndFocusesTheSidebar(t *testing.T) {
	m := shell(t, shellVault(), "/vault/a.md")

	m = send(m, click(5, 1)) // ▸ daily
	if got := tree(m); got[1] != "     2026-10-04" {
		t.Fatalf("after clicking daily, tree = %q", got)
	}
	if !sidebarFocused(m) {
		t.Error("the sidebar does not have focus")
	}

	m = send(m, click(5, 1))
	if got := tree(m); got[1] != " ▸ Projects" {
		t.Errorf("after a second click, tree = %q", got)
	}
}

func TestClickOnANoteOpensIt(t *testing.T) {
	m := shell(t, shellVault(), "/vault/a.md")

	m = send(m, click(5, 5)) // Beta
	if got := m.Path(); got != "/vault/Beta.md" {
		t.Errorf("open Note = %q, want Beta.md", got)
	}
	if sidebarFocused(m) {
		t.Error("the editor does not have focus after opening a Note")
	}
}

func TestClickOnTheSidebarHeaderOnlyFocusesIt(t *testing.T) {
	m := shell(t, shellVault(), "/vault/a.md")
	before := tree(m)

	m = send(m, click(3, 0))
	if !sidebarFocused(m) {
		t.Error("the sidebar does not have focus")
	}
	if got := tree(m); strings.Join(got, "|") != strings.Join(before, "|") {
		t.Errorf("tree changed: %q", got)
	}
}

// The shell's editor pane starts at column 30, after the sidebar.
const edX = 30

func TestClickInTheEditorFocusesItAndMovesTheCursor(t *testing.T) {
	m := keys(shell(t, shellVault(), "/vault/a.md"), ctrlH)

	m = send(m, click(edX+3, 1))
	if got := m.Cursor(); got != (engine.Pos{Line: 1, Col: 3}) {
		t.Errorf("cursor = %+v, want 1:3", got)
	}
	if sidebarFocused(m) {
		t.Error("the editor does not have focus")
	}
}

func TestClickKeepsTheMode(t *testing.T) {
	m := typeKeys(shell(t, shellVault(), "/vault/a.md"), "i")

	m = send(m, click(edX+2, 1))
	if got := m.Cursor(); got != (engine.Pos{Line: 1, Col: 2}) {
		t.Errorf("cursor = %+v, want 1:2", got)
	}
	if got := rows(m)[10]; !strings.HasPrefix(got, " INSERT") {
		t.Errorf("status line = %q, want insert mode", got)
	}

	m = send(m, click(edX+40, 1)) // past the end: insert can sit after it
	if got := m.Cursor(); got != (engine.Pos{Line: 1, Col: len("first note")}) {
		t.Errorf("cursor = %+v, want after the end of line 1", got)
	}
}

func TestClickDropsAHalfTypedCommand(t *testing.T) {
	m := typeKeys(shell(t, shellVault(), "/vault/a.md"), "d")

	m = typeKeys(send(m, click(edX, 1)), "w")
	if got := m.Text(); got != "# Alpha\nfirst note\n" {
		t.Errorf("text = %q, want it untouched", got)
	}
}

func TestClickPastTheTextLandsOnItsEnd(t *testing.T) {
	m := shell(t, shellVault(), "/vault/a.md")

	m = send(m, click(edX+40, 1))
	if got := m.Cursor(); got != (engine.Pos{Line: 1, Col: len("first note") - 1}) {
		t.Errorf("cursor = %+v, want on the last character of line 1", got)
	}

	m = send(m, click(edX+1, 8)) // a ~ row
	if got := m.Cursor(); got.Line != 1 {
		t.Errorf("cursor = %+v, want on the last line", got)
	}
}

// onScreen is the screen cell where text first shows in the editor pane.
func onScreen(t *testing.T, m app.Model, text string) (x, y int) {
	t.Helper()
	for y, row := range rows(m) {
		pane := ansi.Cut(row, edX, 200)
		if i := strings.Index(pane, text); i >= 0 {
			return edX + ansi.StringWidth(pane[:i]), y
		}
	}
	t.Fatalf("%q is not on screen:\n%s", text, screen(m))
	return 0, 0
}

func TestCtrlClickFollowsALink(t *testing.T) {
	m, v := linked(t, "index.md")
	x, y := onScreen(t, m, "beta")

	m = send(m, tea.MouseClickMsg{X: x + 1, Y: y, Button: tea.MouseLeft, Mod: tea.ModCtrl})
	if got := v.rel(t, m); got != "deep/er/beta.md" {
		t.Errorf("open Note = %q, want deep/er/beta.md", got)
	}
}

func TestPlainClickOnALinkOnlyMovesTheCursor(t *testing.T) {
	m, v := linked(t, "index.md")
	x, y := onScreen(t, m, "beta")

	m = send(m, click(x+1, y))
	if got := v.rel(t, m); got != "index.md" {
		t.Errorf("open Note = %q, want index.md still", got)
	}
	if got := m.Cursor(); got.Line != 3 {
		t.Errorf("cursor = %+v, want on line 3", got)
	}
}

// The notes palette at 80×12: the box spans columns 5-74 and its items
// start on row 4 (Alpha, Beta, Deep), with the hint on row 7.

func TestClickOnAPaletteRowChoosesIt(t *testing.T) {
	m, evs := openPalette(t, shell(t, shellVault(), "/vault/a.md"), notesPalette())

	m = send(m, click(20, 5))
	if len(*evs) != 1 || (*evs)[0].Kind != palette.Chosen || (*evs)[0].Item.Text != "Beta" {
		t.Fatalf("events = %+v, want Beta chosen", *evs)
	}
	if strings.Contains(screen(m), "Find Note") {
		t.Error("palette still open after choosing")
	}
}

func TestClickInThePaletteOffTheRowsDoesNothing(t *testing.T) {
	m, evs := openPalette(t, shell(t, shellVault(), "/vault/a.md"), notesPalette())

	for _, y := range []int{1, 2, 3, 7, 8} { // border, input, rule, hint, border
		m = send(m, click(20, y))
	}
	if len(*evs) != 0 {
		t.Errorf("events = %+v, want none", *evs)
	}
	if !strings.Contains(screen(m), "Find Note") {
		t.Error("palette closed")
	}
}

func TestClickOnTheBackdropClosesThePalette(t *testing.T) {
	m, evs := openPalette(t, shell(t, shellVault(), "/vault/a.md"), notesPalette())

	m = send(m, click(2, 9))
	if len(*evs) != 1 || (*evs)[0].Kind != palette.Closed {
		t.Fatalf("events = %+v, want closed", *evs)
	}
	if strings.Contains(screen(m), "Find Note") {
		t.Error("palette still open")
	}
	if got := m.Path(); got != "/vault/a.md" {
		t.Errorf("the click went through to the panes: open Note = %q", got)
	}
}

func TestWheelMovesThePaletteSelection(t *testing.T) {
	m, evs := openPalette(t, shell(t, shellVault(), "/vault/a.md"), notesPalette())

	m = send(m, wheel(20, 5, tea.MouseWheelDown))
	m = send(m, wheel(20, 5, tea.MouseWheelDown))
	m = send(m, wheel(20, 5, tea.MouseWheelUp))
	m = keys(m, enter)
	if len(*evs) != 1 || (*evs)[0].Item.Text != "Beta" {
		t.Errorf("events = %+v, want Beta chosen", *evs)
	}

	m, _ = openPalette(t, resize(m, 80, 6), notesPalette())
	before := screen(m)
	m = send(m, wheel(2, 4, tea.MouseWheelDown)) // over the backdrop: nothing scrolls
	if got := screen(m); got != before {
		t.Errorf("wheel over the backdrop changed the screen:\n%s", got)
	}
}

func motion(x, y int) tea.MouseMotionMsg {
	return tea.MouseMotionMsg{X: x, Y: y, Button: tea.MouseLeft}
}

func borderAt(m app.Model) int { return strings.Index(rows(m)[0], "│") }

func TestDraggingTheSidebarBorderResizesItAndSavesTheWidth(t *testing.T) {
	fsys := shellVault()
	m := shell(t, fsys, "/vault/a.md")
	const state = "/home/u/.local/state/pholio/state.toml"

	m = send(m, click(29, 3))
	m = send(m, motion(35, 4))
	m = send(m, motion(39, 4))
	if got := borderAt(m); got != 39 {
		t.Errorf("while dragging, border at %d, want 39", got)
	}
	m = send(m, release(39, 4))
	data, err := fsys.ReadFile(state)
	if err != nil || !strings.Contains(string(data), "sidebar_width = 40") {
		t.Fatalf("state = %q, %v; want sidebar_width = 40", data, err)
	}

	m = send(m, motion(50, 4)) // no longer dragging
	if got := borderAt(m); got != 39 {
		t.Errorf("after release, motion moved the border to %d", got)
	}

	m = send(m, click(39, 0))
	m = send(m, motion(75, 0))
	m = send(m, release(75, 0))
	if got := borderAt(m); got != 59 {
		t.Errorf("border at %d, want 59: the editor keeps 20 columns", got)
	}
	if data, _ := fsys.ReadFile(state); !strings.Contains(string(data), "sidebar_width = 60") {
		t.Errorf("state = %q, want the width as drawn", data)
	}

	m = send(m, click(59, 0))
	m = send(m, motion(2, 0))
	m = send(m, release(2, 0))
	if got := borderAt(m); got != 15 {
		t.Errorf("border at %d, want the 16-column minimum", got)
	}
}

func wheel(x, y int, b tea.MouseButton) tea.MouseWheelMsg {
	return tea.MouseWheelMsg{X: x, Y: y, Button: b}
}

// longNote opens a 30-line Note, "line 1" to "line 30", with the sidebar
// focused.
func longNote(t *testing.T) app.Model {
	t.Helper()
	fsys := shellVault()
	var text strings.Builder
	for i := 1; i <= 30; i++ {
		fmt.Fprintf(&text, "line %d\n", i)
	}
	if err := fsys.WriteFile("/vault/long.md", []byte(text.String())); err != nil {
		t.Fatal(err)
	}
	return keys(shell(t, fsys, "/vault/long.md"), ctrlH)
}

// firstLine is the editor's top row.
func firstLine(m app.Model) string {
	return strings.TrimSpace(ansi.Cut(rows(m)[0], edX, 200))
}

func TestWheelScrollsTheEditorAndPullsTheCursorAlong(t *testing.T) {
	m := longNote(t)

	m = send(m, wheel(edX+5, 4, tea.MouseWheelDown))
	if got := firstLine(m); got != "line 4" {
		t.Errorf("after wheel down, top row = %q, want line 4", got)
	}
	if got := m.Cursor().Line; got != 6 { // scrolloff rows below the top
		t.Errorf("cursor line = %d, want 6", got)
	}
	if !sidebarFocused(m) {
		t.Error("the wheel moved focus to the editor")
	}

	m = send(m, wheel(edX+5, 4, tea.MouseWheelDown))
	m = send(m, wheel(edX+5, 4, tea.MouseWheelUp))
	if got := firstLine(m); got != "line 4" {
		t.Errorf("after down, down, up, top row = %q, want line 4", got)
	}
	if got := m.Cursor().Line; got != 9 { // pulled to 9, then still on screen
		t.Errorf("cursor line = %d, want 9", got)
	}

	m = send(m, wheel(edX+5, 4, tea.MouseWheelUp))
	m = send(m, wheel(edX+5, 4, tea.MouseWheelUp))
	if got := firstLine(m); got != "line 1" {
		t.Errorf("after scrolling back, top row = %q, want line 1", got)
	}
	if got := m.Cursor().Line; got != 6 { // the bottom row less scrolloff
		t.Errorf("cursor line = %d, want 6", got)
	}
}

func TestWheelScrollsTheSidebarWithoutFocusingIt(t *testing.T) {
	m := resize(shell(t, shellVault(), "/vault/a.md"), 80, 6) // three entries show

	m = send(m, wheel(5, 2, tea.MouseWheelDown))
	if got := strings.Join(tree(m), "|"); got != "   a|   Beta|   notes.txt" {
		t.Errorf("after wheel down, tree = %q", got)
	}
	if sidebarFocused(m) {
		t.Error("the wheel focused the sidebar")
	}

	m = send(m, wheel(5, 2, tea.MouseWheelUp))
	if got := strings.Join(tree(m), "|"); got != " ▸ daily| ▸ Projects| ▸ zettel" {
		t.Errorf("after wheel up, tree = %q", got)
	}
}
