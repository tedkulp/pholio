package app_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/tedkulp/pholio/internal/app"
	"github.com/tedkulp/pholio/internal/config"
	"github.com/tedkulp/pholio/internal/seam/seamtest"
	"github.com/tedkulp/pholio/internal/theme"
)

const (
	userConfig = "/home/u/.config/pholio/config.toml"
	themeDir   = "/home/u/.config/pholio/themes"
)

var dirs = config.Dirs{ConfigHome: "/home/u/.config", StateHome: "/home/u/.local/state"}

// started runs config startup over fsys and opens /vault/a.md with it.
func started(t *testing.T, fsys *seamtest.MemFS) app.Model {
	t.Helper()
	s, err := config.Startup(fsys, dirs, "/home/u", "/vault/a.md")
	if err != nil {
		t.Fatal(err)
	}
	m, err := app.New(app.Deps{FS: fsys}, "/vault/a.md")
	if err != nil {
		t.Fatal(err)
	}
	return resize(m.WithSession(s), 60, 5)
}

func resize(m app.Model, w, h int) app.Model {
	next, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return next.(app.Model)
}

func press(m app.Model, k tea.KeyPressMsg) app.Model {
	next, _ := m.Update(k)
	return next.(app.Model)
}

var (
	f7 = tea.KeyPressMsg{Code: tea.KeyF7}
	f8 = tea.KeyPressMsg{Code: tea.KeyF8}
)

// messageLine is the bottom row, below the status line.
func messageLine(m app.Model) string {
	lines := strings.Split(ansi.Strip(m.View().Content), "\n")
	return strings.TrimRight(lines[len(lines)-1], " ")
}

// drawnWith reports whether the view draws the file name with the status
// file slot of the named theme, loaded the way the app should load it.
func drawnWith(m app.Model, fsys *seamtest.MemFS, name string) bool {
	th, _ := theme.Load(fsys, themeDir, name)
	return strings.Contains(m.View().Content, th.Style(theme.UIStatusFile).Render(" a.md"))
}

func TestStartsWithTheConfiguredTheme(t *testing.T) {
	fsys := seamtest.NewMemFS(map[string]string{userConfig: `theme = "light"` + "\n", "/vault/a.md": "hi\n"})

	m := started(t, fsys)

	if !drawnWith(m, fsys, "light") {
		t.Fatalf("status line not drawn with the light theme:\n%q", m.View().Content)
	}
	if got := messageLine(m); got != "" {
		t.Errorf("message line = %q, want no message", got)
	}
}

func TestStatusLineFillsTheWidth(t *testing.T) {
	fsys := seamtest.NewMemFS(map[string]string{"/vault/a.md": "hi\n"})

	m := started(t, fsys)

	lines := strings.Split(ansi.Strip(m.View().Content), "\n")
	if got := ansi.StringWidth(lines[len(lines)-2]); got != 60 {
		t.Errorf("status line is %d cells wide, want 60", got)
	}
}

func TestStartupReportsThemeAndConfigProblems(t *testing.T) {
	fsys := seamtest.NewMemFS(map[string]string{userConfig: "theme = \"nosuch\"\nbogus = 1\n", "/vault/a.md": ""})

	m := started(t, fsys)

	want := `config: ` + userConfig + `: unknown key "bogus"  theme: no theme named "nosuch"`
	if got := messageLine(resize(m, 200, 5)); got != want {
		t.Errorf("status line =\n %q\nwant\n %q", got, want)
	}
	if !drawnWith(m, fsys, "default") {
		t.Error("a missing theme should fall back to default")
	}
}

func TestF7CyclesThemes(t *testing.T) {
	fsys := seamtest.NewMemFS(map[string]string{themeDir + "/mine.toml": "[ui]\nstatus_file = { fg = \"#123456\" }\n", "/vault/a.md": ""})
	m := started(t, fsys)

	for _, want := range []string{"ansi16", "light", "mine", "default", "ansi16"} {
		m = press(m, f7)
		if got := messageLine(m); got != "theme: "+want {
			t.Fatalf("after F7 status line = %q, want theme %s", got, want)
		}
		if !drawnWith(m, fsys, want) {
			t.Fatalf("after F7 view is not drawn with %s", want)
		}
	}
}

func TestF7ReportsProblemsInTheNextTheme(t *testing.T) {
	fsys := seamtest.NewMemFS(map[string]string{themeDir + "/aaa.toml": "[ui]\nnope = {}\n", "/vault/a.md": ""})
	m := started(t, fsys)

	m = press(m, f7)

	want := `theme: ` + themeDir + `/aaa.toml: unknown slot "ui.nope"`
	if got := messageLine(resize(m, 200, 5)); got != want {
		t.Errorf("status line = %q, want %q", got, want)
	}
}

func TestF8ReloadsConfigAndTheme(t *testing.T) {
	fsys := seamtest.NewMemFS(map[string]string{
		themeDir + "/mine.toml": "[ui]\nstatus_file = { fg = \"#123456\" }\n",
		"/vault/a.md":           "",
	})
	m := started(t, fsys)

	// A changed theme setting takes effect on F8.
	if err := fsys.WriteFile(userConfig, []byte(`theme = "mine"`+"\n")); err != nil {
		t.Fatal(err)
	}
	m = press(m, f8)
	if !drawnWith(m, fsys, "mine") {
		t.Fatal("F8 did not switch to the configured theme")
	}
	if got := messageLine(m); got != "reloaded config and theme" {
		t.Errorf("status line = %q", got)
	}

	// Edits to the current theme file are picked up too.
	if err := fsys.WriteFile(themeDir+"/mine.toml", []byte("[ui]\nstatus_file = { fg = \"#abcdef\" }\n")); err != nil {
		t.Fatal(err)
	}
	m = press(m, f8)
	if !drawnWith(m, fsys, "mine") || !strings.Contains(m.View().Content, "171;205;239") {
		t.Fatalf("F8 did not reload the edited theme:\n%q", m.View().Content)
	}
}

func TestF8KeepsAThemePickedWithF7(t *testing.T) {
	fsys := seamtest.NewMemFS(map[string]string{"/vault/a.md": ""})
	m := started(t, fsys)

	m = press(m, f7) // ansi16
	m = press(m, f8) // config still says default, unchanged

	if !drawnWith(m, fsys, "ansi16") {
		t.Error("F8 dropped the theme picked with F7 although the config's theme did not change")
	}
}

func TestProblemsUseTheErrorSlotAndNoticesTheMessageSlot(t *testing.T) {
	fsys := seamtest.NewMemFS(map[string]string{themeDir + "/aaa.toml": "[ui]\nnope = {}\n", "/vault/a.md": ""})
	m := resize(started(t, fsys), 200, 5)

	m = press(m, f7) // aaa, with a problem
	aaa, _ := theme.Load(fsys, themeDir, "aaa")
	problem := `theme: ` + themeDir + `/aaa.toml: unknown slot "ui.nope"`
	if !strings.Contains(m.View().Content, aaa.Style(theme.UIError).Render(problem)) {
		t.Errorf("problem not drawn with ui.error:\n%q", m.View().Content)
	}

	m = press(m, f7) // ansi16, fine
	ansi16, _ := theme.Load(fsys, themeDir, "ansi16")
	if !strings.Contains(m.View().Content, ansi16.Style(theme.UIMessage).Render("theme: ansi16")) {
		t.Errorf("notice not drawn with ui.message:\n%q", m.View().Content)
	}
}

func TestF8ReportsConfigProblems(t *testing.T) {
	fsys := seamtest.NewMemFS(map[string]string{"/vault/a.md": ""})
	m := started(t, fsys)
	if err := fsys.WriteFile(userConfig, []byte("bogus = 1\n")); err != nil {
		t.Fatal(err)
	}

	m = press(m, f8)

	want := `config: ` + userConfig + `: unknown key "bogus"`
	if got := messageLine(resize(m, 200, 5)); got != want {
		t.Errorf("status line = %q, want %q", got, want)
	}
}
