// Package theme loads pholio's TOML themes: a [palette] of named colours
// plus style slots in the ui, sidebar, overlay, markdown and tasks sections.
// The default, light and ansi16 themes are embedded; user themes live in
// <config home>/pholio/themes/*.toml. A partial theme falls back to the
// default theme slot by slot.
package theme

import (
	"embed"
	"errors"
	"fmt"
	"image/color"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/BurntSushi/toml"

	"github.com/tedkulp/pholio/internal/seam"
)

// DefaultName is the theme every other theme falls back to.
const DefaultName = "default"

//go:embed themes/*.toml
var bundled embed.FS

// Theme is a loaded theme: one lipgloss style per Slot.
type Theme struct {
	name   string
	styles map[Slot]lipgloss.Style
}

// Name is the theme's name: its file name without ".toml".
func (t Theme) Name() string { return t.name }

// Style returns the style for slot, or a plain style if the theme has none.
func (t Theme) Style(s Slot) lipgloss.Style {
	if st, ok := t.styles[s]; ok {
		return st
	}
	return lipgloss.NewStyle()
}

// Dir is the user theme folder under the config home.
func Dir(configHome string) string {
	return filepath.Join(configHome, "pholio", "themes")
}

// Default returns the embedded default theme.
func Default() Theme {
	th, _ := build(DefaultName, mustDecodeDefault(), "")
	return th
}

// Message joins problems into one status-line message, or "" if there are
// none.
func Message(problems []string) string {
	switch len(problems) {
	case 0:
		return ""
	case 1:
		return "theme: " + problems[0]
	}
	return fmt.Sprintf("theme: %d problems: %s", len(problems), strings.Join(problems, "; "))
}

// Load loads the named theme over the default one. A user theme in dir
// wins over an embedded theme of the same name. Problems (a missing theme,
// bad TOML, unknown keys or slots, bad colours) are reported, and whatever
// could not be used falls back to the default theme.
func Load(fsys seam.FS, dir, name string) (Theme, []string) {
	base := mustDecodeDefault()
	data, label, err := read(fsys, dir, name)
	if err != nil {
		th, _ := build(DefaultName, base, "")
		return th, []string{err.Error()}
	}
	if data == nil { // the embedded default itself
		return build(name, base, "")
	}
	var over file
	md, err := toml.Decode(string(data), &over)
	if err != nil {
		th, _ := build(DefaultName, base, "")
		return th, []string{fmt.Sprintf("%s: %v", label, err)}
	}
	var problems []string
	var unknown []string
	for _, k := range md.Undecoded() {
		key := k.String()
		// Report an unknown table once, not every key inside it too.
		if len(unknown) > 0 && strings.HasPrefix(key, unknown[len(unknown)-1]+".") {
			continue
		}
		unknown = append(unknown, key)
		problems = append(problems, fmt.Sprintf("%s: unknown key %q", label, key))
	}
	for k, v := range over.Palette {
		base.Palette[k] = v
	}
	baseSecs := base.sections()
	for _, sec := range sectionNames {
		slots := over.sections()[sec]
		for _, slot := range sortedKeys(slots) {
			if _, ok := baseSecs[sec][slot]; !ok {
				problems = append(problems, fmt.Sprintf("%s: unknown slot %q", label, sec+"."+slot))
				continue
			}
			baseSecs[sec][slot] = slots[slot]
		}
	}
	th, more := build(name, base, label)
	return th, append(problems, more...)
}

// Names lists the available themes: default first, then the rest of the
// embedded and user themes in name order.
func Names(fsys seam.FS, dir string) []string {
	seen := map[string]bool{}
	var names []string
	add := func(file string) {
		n, ok := strings.CutSuffix(file, ".toml")
		if ok && n != "" && !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}
	ents, _ := bundled.ReadDir("themes")
	for _, e := range ents {
		add(e.Name())
	}
	if dir != "" {
		ents, _ = fsys.ReadDir(dir) // no user theme folder is fine
		for _, e := range ents {
			if !e.IsDir() {
				add(e.Name())
			}
		}
	}
	sort.Slice(names, func(i, j int) bool {
		if names[i] == DefaultName || names[j] == DefaultName {
			return names[i] == DefaultName
		}
		return names[i] < names[j]
	})
	return names
}

// read returns the overlay file for name and a label for problems. It
// returns nil data for the embedded default with no user override.
func read(fsys seam.FS, dir, name string) ([]byte, string, error) {
	if name == "" || strings.ContainsAny(name, `/\`) {
		return nil, "", fmt.Errorf("bad theme name %q", name)
	}
	if dir != "" {
		p := filepath.Join(dir, name+".toml")
		data, err := fsys.ReadFile(p)
		if err == nil {
			return data, p, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, "", fmt.Errorf("%s: %w", p, err)
		}
	}
	if name == DefaultName {
		return nil, "", nil
	}
	data, err := bundled.ReadFile("themes/" + name + ".toml")
	if err != nil {
		return nil, "", fmt.Errorf("no theme named %q", name)
	}
	return data, name + " (built-in)", nil
}

type styleDef struct {
	Fg        string `toml:"fg"`
	Bg        string `toml:"bg"`
	Bold      bool   `toml:"bold"`
	Italic    bool   `toml:"italic"`
	Underline bool   `toml:"underline"`
	Strike    bool   `toml:"strike"`
	Faint     bool   `toml:"faint"`
	Reverse   bool   `toml:"reverse"`
}

type file struct {
	Palette  map[string]string   `toml:"palette"`
	UI       map[string]styleDef `toml:"ui"`
	Sidebar  map[string]styleDef `toml:"sidebar"`
	Overlay  map[string]styleDef `toml:"overlay"`
	Markdown map[string]styleDef `toml:"markdown"`
	Tasks    map[string]styleDef `toml:"tasks"`
}

var sectionNames = []string{"ui", "sidebar", "overlay", "markdown", "tasks"}

func (f *file) sections() map[string]map[string]styleDef {
	return map[string]map[string]styleDef{
		"ui": f.UI, "sidebar": f.Sidebar, "overlay": f.Overlay, "markdown": f.Markdown, "tasks": f.Tasks,
	}
}

func mustDecodeDefault() file {
	data, err := bundled.ReadFile("themes/" + DefaultName + ".toml")
	if err != nil {
		panic(err)
	}
	var f file
	if _, err := toml.Decode(string(data), &f); err != nil {
		panic(fmt.Sprintf("embedded default theme: %v", err))
	}
	return f
}

// build resolves palette names and turns every slot into a style. Bad
// colours are reported against label and left unset.
func build(name string, f file, label string) (Theme, []string) {
	var problems []string
	palette := map[string]color.Color{}
	for _, k := range sortedKeys(f.Palette) {
		c, err := ParseColor(f.Palette[k])
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: palette.%s: %v", label, k, err))
			continue
		}
		palette[k] = c
	}
	resolve := func(where, v string) color.Color {
		if c, ok := palette[v]; ok {
			return c
		}
		if _, ok := f.Palette[v]; ok {
			return nil // a bad palette entry, already reported
		}
		c, err := ParseColor(v)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %s: %v (not a palette name, hex or ANSI number)", label, where, err))
			return nil
		}
		return c
	}
	th := Theme{name: name, styles: map[Slot]lipgloss.Style{}}
	secs := f.sections()
	for _, sec := range sectionNames {
		for _, slot := range sortedKeys(secs[sec]) {
			d := secs[sec][slot]
			key := sec + "." + slot
			st := lipgloss.NewStyle().Bold(d.Bold).Italic(d.Italic).Underline(d.Underline).
				Strikethrough(d.Strike).Faint(d.Faint).Reverse(d.Reverse)
			if d.Fg != "" {
				if c := resolve(key+".fg", d.Fg); c != nil {
					st = st.Foreground(c)
				}
			}
			if d.Bg != "" {
				if c := resolve(key+".bg", d.Bg); c != nil {
					st = st.Background(c)
				}
			}
			th.styles[Slot(key)] = st
		}
	}
	return th, problems
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
