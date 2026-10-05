// PROTOTYPE — throwaway code for wayfinder ticket "App shell prototype".
// Three shell variants (F5/F6 to switch), three themes (F7, F8 reloads from disk).
// Run from this directory: go run . [vault-dir]   (no dir: a scratch copy of ./vault)

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"pholio/prototypes/vim/engine"
)

type osFS struct{}

func (osFS) ReadFile(p string) (string, error) { b, err := os.ReadFile(p); return string(b), err }
func (osFS) WriteFile(p, d string) error       { return os.WriteFile(p, []byte(d), 0o644) }

const (
	vSplit = iota // A: persistent sidebar, vim ctrl-w focus, centered modals
	vDrawer       // B: hidden drawer on a leader key, side-panel overlays, cmdline prompt
	vPalette      // C: persistent sidebar, tab focus, top-anchored command palette overlays
	nVariants
)

var variantNames = []string{"A · Split + ctrl-w + modals", "B · Drawer + side panel", "C · Tab focus + palette"}
var variantHints = []string{
	"ctrl+w h/l/w focus · ctrl+w e sidebar · ctrl+w </> width · spc t tasks · spc z zettel",
	"spc e drawer · esc closes · spc t tasks · spc z zettel",
	"tab focus · spc e sidebar · spc t tasks (type to filter) · spc z zettel",
}

const (
	focusEditor = iota
	focusSidebar
)

const (
	ovNone = iota
	ovTasks
	ovZettel
)

type model struct {
	w, h    int
	variant int
	vault   string

	e       *engine.Engine
	engines map[string]*engine.Engine
	top     int

	focus      int
	sidebarOn  bool
	sideW      int
	expanded   map[string]bool
	showHidden bool
	sel        int
	sideTop    int

	themes   []string
	themeIdx int
	th       *theme
	warn     []string

	overlay int
	tasks   []task
	taskSel int
	filter  string
	input   string

	prefix string // pending "spc" or "ctrl+w"
	msg    string
}

func (m *model) Init() tea.Cmd { return nil }

func (m *model) setVariant(v int) {
	m.variant = (v + nVariants) % nVariants
	m.overlay, m.prefix, m.focus = ovNone, "", focusEditor
	m.sidebarOn = m.variant != vDrawer
	m.msg = "variant " + variantNames[m.variant]
}

func (m *model) setTheme(i int) {
	m.themeIdx = (i + len(m.themes)) % len(m.themes)
	m.th, m.warn = loadTheme(m.themes[m.themeIdx])
	m.msg = "theme " + m.th.name
	if len(m.warn) > 0 {
		m.msg = "theme " + m.th.name + ": " + strings.Join(m.warn, "; ")
	}
}

func (m *model) open(path string) {
	if e, ok := m.engines[path]; ok {
		m.e = e
	} else {
		m.e = engine.New(osFS{}, path)
		m.engines[path] = m.e
	}
	m.e.PageLines = m.bodyH()
	m.top = 0
	for d := filepath.Dir(path); strings.HasPrefix(d, m.vault) && d != m.vault; d = filepath.Dir(d) {
		m.expanded[d] = true
	}
}

func (m *model) bodyH() int { return max(1, m.h-3) }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
		for _, e := range m.engines {
			e.PageLines = m.bodyH()
		}
	case tea.PasteMsg:
		if m.overlay == ovNone && m.focus == focusEditor {
			m.e.Paste(msg.Content)
		}
	case tea.KeyPressMsg:
		k := msg.Key()
		name := msg.Keystroke()
		if k.Text != "" && k.Mod&(tea.ModCtrl|tea.ModAlt) == 0 {
			name = k.Text
		}
		return m, m.key(name, k.Text)
	}
	return m, nil
}

func (m *model) key(name, text string) tea.Cmd {
	switch name {
	case "ctrl+q":
		return tea.Quit
	case "f5":
		m.setVariant(m.variant - 1)
		return nil
	case "f6":
		m.setVariant(m.variant + 1)
		return nil
	case "f7":
		m.setTheme(m.themeIdx + 1)
		return nil
	case "f8":
		m.setTheme(m.themeIdx)
		return nil
	}
	if m.overlay != ovNone {
		m.overlayKey(name, text)
		return nil
	}
	if m.prefix != "" {
		p := m.prefix
		m.prefix = ""
		m.prefixKey(p, name)
		return nil
	}
	free := m.focus == focusSidebar || m.e.Mode == engine.Normal && m.e.PendingKeys() == ""
	if free {
		switch {
		case name == " ":
			m.prefix = "spc"
			return nil
		case name == "ctrl+w" && m.variant == vSplit:
			m.prefix = "ctrl+w"
			return nil
		case m.variant == vPalette && (name == "tab" || name == "ctrl+h" || name == "ctrl+l"):
			m.moveFocus(map[string]int{"tab": 1 - m.focus, "ctrl+h": focusSidebar, "ctrl+l": focusEditor}[name])
			return nil
		}
	}
	if m.focus == focusSidebar {
		m.sidebarKey(name)
		return nil
	}
	m.msg = ""
	m.e.Feed(name)
	if m.e.Quit {
		return tea.Quit
	}
	return nil
}

func (m *model) moveFocus(f int) {
	if f == focusSidebar && !m.sidebarOn {
		m.msg = "sidebar is hidden"
		return
	}
	m.focus = f
}

func (m *model) toggleSidebar() {
	m.sidebarOn = !m.sidebarOn
	if m.sidebarOn {
		m.focus = focusSidebar
	} else {
		m.focus = focusEditor
	}
}

func (m *model) prefixKey(p, name string) {
	if p == "spc" {
		switch name {
		case "e":
			m.toggleSidebar()
		case "t":
			m.tasks, m.taskSel, m.filter = scanTasks(m.vault), 0, ""
			m.overlay = ovTasks
		case "z":
			m.input, m.overlay = "", ovZettel
		default:
			m.msg = "spc " + name + ": unbound"
		}
		return
	}
	switch name { // ctrl+w, variant A
	case "h", "ctrl+h":
		m.moveFocus(focusSidebar)
	case "l", "ctrl+l":
		m.moveFocus(focusEditor)
	case "w", "ctrl+w":
		m.moveFocus(1 - m.focus)
	case "e":
		m.toggleSidebar()
	case "<":
		m.sideW = max(16, m.sideW-4)
	case ">":
		m.sideW = min(80, m.sideW+4)
	case "=":
		m.sideW = 30
	default:
		m.msg = "ctrl+w " + name + ": unbound"
	}
}

func (m *model) sidebarKey(name string) {
	nodes := m.visibleNodes()
	if len(nodes) == 0 {
		return
	}
	m.sel = min(m.sel, len(nodes)-1)
	n := nodes[m.sel]
	switch name {
	case "j", "down":
		m.sel = min(m.sel+1, len(nodes)-1)
	case "k", "up":
		m.sel = max(m.sel-1, 0)
	case "g", "home":
		m.sel = 0
	case "G", "end":
		m.sel = len(nodes) - 1
	case "l", "right", "enter", "o":
		if n.dir {
			if name == "enter" || name == "o" {
				m.expanded[n.path] = !m.expanded[n.path]
			} else {
				m.expanded[n.path] = true
			}
			return
		}
		if !strings.HasSuffix(n.name, ".md") {
			m.msg = n.name + ": not a note"
			return
		}
		m.open(n.path)
		m.focus = focusEditor
		if m.variant == vDrawer {
			m.sidebarOn = false
		}
	case "h", "left":
		if n.dir && m.expanded[n.path] {
			m.expanded[n.path] = false
			return
		}
		parent := filepath.Dir(n.path)
		for i := m.sel - 1; i >= 0; i-- {
			if nodes[i].path == parent {
				m.sel = i
				m.expanded[parent] = false
				break
			}
		}
	case ".":
		m.showHidden = !m.showHidden
		m.msg = fmt.Sprintf("hidden files: %v", m.showHidden)
	case "<":
		m.sideW = max(16, m.sideW-4)
	case ">":
		m.sideW = min(80, m.sideW+4)
	case "esc":
		if m.variant == vDrawer {
			m.sidebarOn = false
		}
		m.focus = focusEditor
	}
}

func (m *model) filteredTasks() []task {
	if m.filter == "" {
		return m.tasks
	}
	var out []task
	f := strings.ToLower(m.filter)
	for _, t := range m.tasks {
		if strings.Contains(strings.ToLower(t.text+" "+t.file), f) {
			out = append(out, t)
		}
	}
	return out
}

func (m *model) overlayKey(name, text string) {
	if name == "esc" || name == "ctrl+c" {
		m.overlay = ovNone
		return
	}
	if m.overlay == ovZettel {
		switch name {
		case "enter":
			m.createZettel()
		case "backspace":
			if r := []rune(m.input); len(r) > 0 {
				m.input = string(r[:len(r)-1])
			}
		case "ctrl+u":
			m.input = ""
		default:
			m.input += text
		}
		return
	}
	ts := m.filteredTasks()
	typing := m.variant == vPalette
	switch {
	case name == "down" || name == "ctrl+n" || !typing && name == "j":
		m.taskSel = min(m.taskSel+1, len(ts)-1)
	case name == "up" || name == "ctrl+p" || !typing && name == "k":
		m.taskSel = max(m.taskSel-1, 0)
	case name == "enter":
		if m.taskSel < len(ts) {
			t := ts[m.taskSel]
			m.open(filepath.Join(m.vault, t.file))
			m.e.Cur = engine.Pos{Line: t.line}
			m.focus, m.overlay = focusEditor, ovNone
			if m.variant == vDrawer {
				m.sidebarOn = false
			}
		}
	case !typing && name == "q":
		m.overlay = ovNone
	case typing && name == "backspace":
		if r := []rune(m.filter); len(r) > 0 {
			m.filter = string(r[:len(r)-1])
		}
		m.taskSel = 0
	case typing && text != "":
		m.filter += text
		m.taskSel = 0
	}
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func zettelName(title string) string {
	slug := strings.Trim(slugRe.ReplaceAllString(strings.ToLower(title), "-"), "-")
	return time.Now().Format("200601021504") + "-" + slug
}

func (m *model) createZettel() {
	title := strings.TrimSpace(m.input)
	if title == "" {
		m.msg = "empty title, nothing created"
		m.overlay = ovNone
		return
	}
	name := zettelName(title)
	origin := strings.TrimSuffix(filepath.Base(m.e.Path), ".md")
	dir := filepath.Join(m.vault, "zettel")
	os.MkdirAll(dir, 0o755)
	body := fmt.Sprintf("# %s\n\n\n\nOrigin: [[%s]]\n", title, origin)
	os.WriteFile(filepath.Join(dir, name+".md"), []byte(body), 0o644)
	m.expanded[dir] = true
	if m.e.Mode == engine.Normal {
		m.e.Feed("a")
		m.e.Paste("[[" + name + "]]")
		m.e.Feed("esc")
	}
	m.overlay, m.focus = ovNone, focusEditor
	m.msg = "created zettel/" + name + ".md, linked at cursor"
}

func main() {
	flag.Parse()
	vault := flag.Arg(0)
	if vault == "" {
		vault = filepath.Join(os.TempDir(), "pholio-shell-proto-vault")
		os.RemoveAll(vault)
		copyDir("vault", vault)
	}
	vault, _ = filepath.Abs(vault)
	m := &model{vault: vault, engines: map[string]*engine.Engine{}, expanded: map[string]bool{}, sideW: 30, themes: themeNames()}
	m.setVariant(vSplit)
	m.setTheme(0)
	m.msg = "F5/F6 switch variant · F7 theme · F8 reload theme · ctrl+q quit"
	start := filepath.Join(vault, "daily", time.Now().Format("2006-01-02")+".md")
	if _, err := os.Stat(start); err != nil {
		start = filepath.Join(vault, "inbox.md")
	}
	m.open(start)
	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
