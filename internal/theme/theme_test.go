package theme_test

import (
	"image/color"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/tedkulp/pholio/internal/seam/seamtest"
	"github.com/tedkulp/pholio/internal/theme"
)

const dir = "/home/u/.config/pholio/themes"

func hex(s string) color.Color {
	c, err := theme.ParseColor(s)
	if err != nil {
		panic(err)
	}
	return c
}

func load(t *testing.T, files map[string]string, name string) (theme.Theme, []string) {
	t.Helper()
	return theme.Load(seamtest.NewMemFS(files), dir, name)
}

func assertProblems(t *testing.T, got, want []string) {
	t.Helper()
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("problems:\n got %q\nwant %q", got, want)
	}
}

func TestLoadDefault(t *testing.T) {
	th, problems := load(t, nil, "default")

	assertProblems(t, problems, nil)
	if th.Name() != "default" {
		t.Errorf("Name() = %q, want default", th.Name())
	}
	st := th.Style(theme.UIStatusline)
	if got := st.GetForeground(); got != hex("#a6adc8") {
		t.Errorf("statusline fg = %v, want subtext #a6adc8", got)
	}
	if got := st.GetBackground(); got != hex("#313244") {
		t.Errorf("statusline bg = %v, want surface #313244", got)
	}
	if !th.Style(theme.MarkdownTaskDone).GetStrikethrough() {
		t.Error("markdown.task_done is not struck through")
	}
	if !th.Style(theme.UIStatusFile).GetBold() {
		t.Error("ui.status_file is not bold")
	}
}

func TestPartialThemeFallsBackSlotBySlot(t *testing.T) {
	th, problems := load(t, nil, "light")

	assertProblems(t, problems, nil)
	if th.Name() != "light" {
		t.Errorf("Name() = %q, want light", th.Name())
	}
	// light sets overlay.backdrop itself, replacing default's whole slot...
	bd := th.Style(theme.OverlayBackdrop)
	if got := bd.GetForeground(); got != hex("#ccd0da") {
		t.Errorf("backdrop fg = %v, want light surface #ccd0da", got)
	}
	if bd.GetFaint() {
		t.Error("backdrop is faint, but light's slot replaces default's whole slot")
	}
	// ...and takes ui.statusline from default, resolved against its own palette.
	if got := th.Style(theme.UIStatusline).GetForeground(); got != hex("#5c5f77") {
		t.Errorf("statusline fg = %v, want light subtext #5c5f77", got)
	}
}

func TestUserThemeLoadsFromThemeDir(t *testing.T) {
	th, problems := load(t, map[string]string{
		dir + "/mine.toml": "[palette]\naccent = \"#ff0000\"\n\n[ui]\nerror = { fg = \"12\", underline = true }\n",
	}, "mine")

	assertProblems(t, problems, nil)
	if got := th.Style(theme.UIError).GetForeground(); got != hex("12") {
		t.Errorf("ui.error fg = %v, want ANSI 12", got)
	}
	if !th.Style(theme.UIError).GetUnderline() {
		t.Error("ui.error is not underlined")
	}
	if got := th.Style(theme.SidebarDir).GetForeground(); got != hex("#ff0000") {
		t.Errorf("sidebar.dir fg = %v, want the overridden accent #ff0000", got)
	}
}

func TestUserThemeWinsOverBuiltIn(t *testing.T) {
	th, _ := load(t, map[string]string{dir + "/light.toml": "[ui]\nbase = { fg = \"#010203\" }\n"}, "light")
	if got := th.Style(theme.UIBase).GetForeground(); got != hex("#010203") {
		t.Errorf("ui.base fg = %v, want the user's #010203", got)
	}
	if got := th.Style(theme.UIStatusline).GetForeground(); got != hex("#a6adc8") {
		t.Errorf("statusline fg = %v, want default's palette (the user light.toml has none)", got)
	}

	def, _ := load(t, map[string]string{dir + "/default.toml": "[ui]\nmessage = { fg = \"#040506\" }\n"}, "default")
	if got := def.Style(theme.UIMessage).GetForeground(); got != hex("#040506") {
		t.Errorf("default ui.message fg = %v, want the user's #040506", got)
	}
}

func TestUnknownSlotsAndKeysAreReported(t *testing.T) {
	th, problems := load(t, map[string]string{
		dir + "/odd.toml": `
[ui]
base = { fg = "red", fgg = "x" }
nope = { bold = true }

[sidebarr]
dir = { fg = "1" }
`,
	}, "odd")

	assertProblems(t, problems, []string{
		dir + `/odd.toml: unknown key "ui.base.fgg"`,
		dir + `/odd.toml: unknown key "sidebarr"`,
		dir + `/odd.toml: unknown slot "ui.nope"`,
	})
	if got := th.Style(theme.UIBase).GetForeground(); got != hex("#f38ba8") {
		t.Errorf("ui.base fg = %v, want palette red #f38ba8", got)
	}
}

func TestBadColoursAreReportedAndLeftUnset(t *testing.T) {
	th, problems := load(t, map[string]string{
		dir + "/bad.toml": `
[palette]
accent = "blue"

[ui]
base = { fg = "nosuch", bold = true }
`,
	}, "bad")

	assertProblems(t, problems, []string{
		dir + `/bad.toml: palette.accent: bad colour "blue"`,
		dir + `/bad.toml: ui.base.fg: bad colour "nosuch" (not a palette name, hex or ANSI number)`,
	})
	base := th.Style(theme.UIBase)
	if got := base.GetForeground(); got != (lipgloss.NoColor{}) {
		t.Errorf("ui.base fg = %v, want unset", got)
	}
	if !base.GetBold() {
		t.Error("ui.base lost its other attributes")
	}
	// sidebar.dir uses accent, whose entry is bad: no colour, no second report.
	if got := th.Style(theme.SidebarDir).GetForeground(); got != (lipgloss.NoColor{}) {
		t.Errorf("sidebar.dir fg = %v, want unset", got)
	}
}

func TestMissingThemeFallsBackToDefault(t *testing.T) {
	th, problems := load(t, nil, "nosuch")

	assertProblems(t, problems, []string{`no theme named "nosuch"`})
	if th.Name() != "default" {
		t.Errorf("Name() = %q, want default", th.Name())
	}
	if got := th.Style(theme.UIStatusline).GetForeground(); got != hex("#a6adc8") {
		t.Errorf("statusline fg = %v, want default's", got)
	}
}

func TestBrokenTOMLFallsBackToDefault(t *testing.T) {
	th, problems := load(t, map[string]string{dir + "/broken.toml": "[ui\n"}, "broken")

	if len(problems) != 1 || !strings.HasPrefix(problems[0], dir+"/broken.toml: ") {
		t.Fatalf("problems = %q, want one naming the file", problems)
	}
	if th.Name() != "default" {
		t.Errorf("Name() = %q, want default", th.Name())
	}
}

func TestBundledThemesLoadCleanly(t *testing.T) {
	for _, name := range []string{"default", "light", "ansi16"} {
		th, problems := load(t, nil, name)
		if len(problems) != 0 {
			t.Errorf("%s: problems = %q", name, problems)
		}
		if th.Name() != name {
			t.Errorf("Name() = %q, want %q", th.Name(), name)
		}
	}
	if got := theme.Default().Style(theme.UIBase).GetForeground(); got != hex("#cdd6f4") {
		t.Errorf("Default() ui.base fg = %v, want #cdd6f4", got)
	}
}

func TestNames(t *testing.T) {
	got := theme.Names(seamtest.NewMemFS(map[string]string{
		dir + "/zebra.toml": "",
		dir + "/light.toml": "",
		dir + "/aaa.toml":   "",
		dir + "/notes.txt":  "",
		dir + "/sub/x.toml": "",
	}), dir)
	if want := "default,aaa,ansi16,light,zebra"; strings.Join(got, ",") != want {
		t.Errorf("Names() = %q, want %s", got, want)
	}

	if got := theme.Names(seamtest.NewMemFS(nil), dir); strings.Join(got, ",") != "default,ansi16,light" {
		t.Errorf("Names() with no user folder = %q", got)
	}
}

func TestMessage(t *testing.T) {
	if got := theme.Message(nil); got != "" {
		t.Errorf("Message(nil) = %q", got)
	}
	if got := theme.Message([]string{"a"}); got != "theme: a" {
		t.Errorf("Message(one) = %q", got)
	}
	if got := theme.Message([]string{"a", "b"}); got != "theme: 2 problems: a; b" {
		t.Errorf("Message(two) = %q", got)
	}
}
