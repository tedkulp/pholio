package main

// PROTOTYPE: TOML theme loading. A theme is a palette plus named style slots
// grouped by section. Partial themes fall back to default.toml slot by slot.

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/BurntSushi/toml"
)

//go:embed themes/*.toml
var bundledThemes embed.FS

type styleDef struct {
	Fg, Bg                                          string
	Bold, Italic, Underline, Strike, Faint, Reverse bool
}

type themeFile struct {
	Name     string
	Palette  map[string]string
	UI       map[string]styleDef `toml:"ui"`
	Sidebar  map[string]styleDef
	Overlay  map[string]styleDef
	Markdown map[string]styleDef
	Tasks    map[string]styleDef
}

func (t *themeFile) sections() map[string]map[string]styleDef {
	return map[string]map[string]styleDef{
		"ui": t.UI, "sidebar": t.Sidebar, "overlay": t.Overlay, "markdown": t.Markdown, "tasks": t.Tasks,
	}
}

type theme struct {
	name   string
	styles map[string]lipgloss.Style // "section.slot"
}

func (t *theme) s(key string) lipgloss.Style {
	if st, ok := t.styles[key]; ok {
		return st
	}
	return lipgloss.NewStyle()
}

// readTheme prefers the file on disk (so F8 picks up edits) over the bundled copy.
func readTheme(name string) ([]byte, error) {
	if b, err := os.ReadFile(filepath.Join("themes", name+".toml")); err == nil {
		return b, nil
	}
	return bundledThemes.ReadFile("themes/" + name + ".toml")
}

func themeNames() []string {
	seen := map[string]bool{}
	files, _ := filepath.Glob("themes/*.toml")
	ents, _ := bundledThemes.ReadDir("themes")
	for _, e := range ents {
		files = append(files, e.Name())
	}
	var names []string
	for _, f := range files {
		n := strings.TrimSuffix(filepath.Base(f), ".toml")
		if !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}
	sort.Slice(names, func(i, j int) bool { return names[i] == "default" || names[j] != "default" && names[i] < names[j] })
	return names
}

func decode(name string) (*themeFile, []string, error) {
	b, err := readTheme(name)
	if err != nil {
		return nil, nil, err
	}
	var tf themeFile
	md, err := toml.Decode(string(b), &tf)
	if err != nil {
		return nil, nil, err
	}
	var warn []string
	for _, k := range md.Undecoded() {
		warn = append(warn, "unknown key "+k.String())
	}
	return &tf, warn, nil
}

func loadTheme(name string) (*theme, []string) {
	base, warn, err := decode("default")
	if err != nil {
		return &theme{name: "broken", styles: map[string]lipgloss.Style{}}, []string{err.Error()}
	}
	if name != "default" {
		over, w2, err := decode(name)
		if err != nil {
			warn = append(warn, err.Error())
		} else {
			warn = append(warn, w2...)
			if over.Name != "" {
				base.Name = over.Name
			}
			for k, v := range over.Palette {
				base.Palette[k] = v
			}
			bs := base.sections()
			for sec, slots := range over.sections() {
				for slot, def := range slots {
					if _, ok := bs[sec][slot]; !ok {
						warn = append(warn, fmt.Sprintf("unknown slot %s.%s", sec, slot))
						continue
					}
					bs[sec][slot] = def
				}
			}
		}
	}
	color := func(c string) string {
		if p, ok := base.Palette[c]; ok {
			return p
		}
		return c
	}
	t := &theme{name: base.Name, styles: map[string]lipgloss.Style{}}
	for sec, slots := range base.sections() {
		for slot, d := range slots {
			st := lipgloss.NewStyle().Bold(d.Bold).Italic(d.Italic).Underline(d.Underline).
				Strikethrough(d.Strike).Faint(d.Faint).Reverse(d.Reverse)
			if d.Fg != "" {
				st = st.Foreground(lipgloss.Color(color(d.Fg)))
			}
			if d.Bg != "" {
				st = st.Background(lipgloss.Color(color(d.Bg)))
			}
			t.styles[sec+"."+slot] = st
		}
	}
	return t, warn
}
