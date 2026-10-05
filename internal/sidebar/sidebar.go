// Package sidebar is the file-tree pane: a navigable view of the Vault's
// folders and files. It reads directories through seam.FS only when the
// tree changes shape (expand, collapse, the dotfile toggle, Refresh), so
// drawing never touches the disk.
package sidebar

import (
	"io/fs"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/tedkulp/pholio/internal/seam"
	"github.com/tedkulp/pholio/internal/theme"
)

// Node is one visible row of the tree.
type Node struct {
	Path   string // absolute OS path
	Name   string // file or folder name, with extension
	Dir    bool
	Hidden bool // a dotfile or dotfolder
	Depth  int  // 0 for entries directly in the Vault
}

// Note reports whether the node is a Markdown Note.
func (n Node) Note() bool { return !n.Dir && strings.HasSuffix(n.Name, ".md") }

// Label is how the node is shown: Notes without their ".md".
func (n Node) Label() string {
	if n.Note() {
		return strings.TrimSuffix(n.Name, ".md")
	}
	return n.Name
}

// EventKind says what a key did that the host has to act on.
type EventKind int

// Events.
const (
	None    EventKind = iota
	Open              // open the Note at Event.Path
	Message           // show Event.Text
)

// Event is the outcome of a key for the host.
type Event struct {
	Kind EventKind
	Path string
	Text string
}

// Model is the tree. It is a value: use the Model that methods return.
type Model struct {
	fs         seam.FS
	root       string
	expanded   map[string]bool // never mutated in place; cloned on change
	showHidden bool
	nodes      []Node
	sel, top   int
	h          int // rows available for entries (header excluded)
}

// New returns the tree over the Vault at root, with every folder collapsed.
func New(fsys seam.FS, root string) Model {
	m := Model{fs: fsys, root: root, expanded: map[string]bool{}, h: 1}
	return m.rebuild()
}

// Root is the Vault folder the tree shows.
func (m Model) Root() string { return m.root }

// Nodes are the visible rows, top to bottom.
func (m Model) Nodes() []Node { return m.nodes }

// Selected is the node under the selection, if the tree is not empty.
func (m Model) Selected() (Node, bool) {
	if m.sel < len(m.nodes) {
		return m.nodes[m.sel], true
	}
	return Node{}, false
}

// ShowHidden reports whether dotfiles are shown.
func (m Model) ShowHidden() bool { return m.showHidden }

// SetHeight sets how many rows the pane has, header included.
func (m Model) SetHeight(h int) Model {
	m.h = max(1, h-1)
	return m.clamp()
}

// Refresh rereads the expanded folders, for after files change on disk.
// The selection stays on the same path when it still exists.
func (m Model) Refresh() Model {
	var path string
	if n, ok := m.Selected(); ok {
		path = n.Path
	}
	m = m.rebuild()
	return m.selectPath(path)
}

// Reveal expands the folders above path and selects it.
func (m Model) Reveal(path string) Model {
	ex := maps.Clone(m.expanded)
	for d := filepath.Dir(path); d != m.root && strings.HasPrefix(d, m.root+string(filepath.Separator)); d = filepath.Dir(d) {
		ex[d] = true
	}
	m.expanded = ex
	m = m.rebuild()
	return m.selectPath(path)
}

func (m Model) selectPath(path string) Model {
	for i, n := range m.nodes {
		if n.Path == path {
			m.sel = i
		}
	}
	return m.clamp()
}

// Update handles one key, named as the editor names keys ("j", "enter").
func (m Model) Update(key string) (Model, Event) {
	n, ok := m.Selected()
	switch key {
	case "j", "down":
		m.sel++
	case "k", "up":
		m.sel--
	case "g", "home":
		m.sel = 0
	case "G", "end":
		m.sel = len(m.nodes) - 1
	case "l", "right", "enter":
		if !ok {
			break
		}
		if n.Dir {
			open := !m.expanded[n.Path] || key != "enter"
			m = m.setExpanded(n.Path, open)
			break
		}
		if !n.Note() {
			return m, Event{Kind: Message, Text: n.Name + ": not a Note"}
		}
		return m, Event{Kind: Open, Path: n.Path}
	case "h", "left":
		if !ok {
			break
		}
		if n.Dir && m.expanded[n.Path] {
			m = m.setExpanded(n.Path, false)
			break
		}
		if parent := filepath.Dir(n.Path); parent != m.root {
			m = m.setExpanded(parent, false).selectPath(parent)
		}
	case ".":
		m.showHidden = !m.showHidden
		m = m.Refresh()
		text := "hiding dotfiles"
		if m.showHidden {
			text = "showing dotfiles"
		}
		return m.clamp(), Event{Kind: Message, Text: text}
	}
	return m.clamp(), Event{}
}

// Click acts on screen row row of the pane, as View draws it: a folder
// opens or closes and a Note is opened, as enter does. Row 0 is the
// header, which does nothing.
func (m Model) Click(row int) (Model, Event) {
	i := m.top + row - 1
	if row < 1 || i >= len(m.nodes) {
		return m, Event{}
	}
	m.sel = i
	return m.Update("enter")
}

// Scroll moves the view n rows down (up when n < 0). The selection is
// pulled along when it would leave the view.
func (m Model) Scroll(n int) Model {
	m.top = max(0, min(m.top+n, len(m.nodes)-m.h))
	m.sel = max(m.top, min(m.sel, m.top+m.h-1))
	return m.clamp()
}

func (m Model) setExpanded(dir string, open bool) Model {
	ex := maps.Clone(m.expanded)
	if open {
		ex[dir] = true
	} else {
		delete(ex, dir)
	}
	m.expanded = ex
	return m.rebuild()
}

// clamp keeps the selection on a node and in view.
func (m Model) clamp() Model {
	m.sel = max(0, min(m.sel, len(m.nodes)-1))
	if m.sel < m.top {
		m.top = m.sel
	}
	if m.sel >= m.top+m.h {
		m.top = m.sel - m.h + 1
	}
	m.top = max(0, min(m.top, len(m.nodes)-m.h))
	return m
}

// rebuild walks the root and every expanded folder.
func (m Model) rebuild() Model {
	var out []Node
	var walk func(dir string, depth int)
	walk = func(dir string, depth int) {
		ents, err := m.fs.ReadDir(dir)
		if err != nil {
			return
		}
		ents = slices.Clone(ents)
		slices.SortStableFunc(ents, func(a, b fs.DirEntry) int {
			if a.IsDir() != b.IsDir() {
				if a.IsDir() {
					return -1
				}
				return 1
			}
			return strings.Compare(strings.ToLower(a.Name()), strings.ToLower(b.Name()))
		})
		for _, e := range ents {
			name := e.Name()
			hidden := strings.HasPrefix(name, ".")
			if hidden && !m.showHidden || depth == 0 && name == ".pholio" {
				continue
			}
			p := filepath.Join(dir, name)
			out = append(out, Node{Path: p, Name: name, Dir: e.IsDir(), Hidden: hidden, Depth: depth})
			if e.IsDir() && m.expanded[p] {
				walk(p, depth+1)
			}
		}
	}
	walk(m.root, 0)
	m.nodes = out
	return m.clamp()
}

// View draws the pane w cells wide and h rows tall, including its right
// border. open is the path of the Note in the editor.
func (m Model) View(th theme.Theme, w, h int, focused bool, open string) string {
	border := th.Style(theme.UIPaneBorder)
	if focused {
		border = th.Style(theme.UIPaneBorderFocus)
	}
	inner := max(0, w-1)
	bar := border.Render("│")
	head := " " + filepath.Base(m.root)
	if m.showHidden {
		head += " (all)"
	}
	rows := []string{th.Style(theme.SidebarHeader).Render(fit(head, inner)) + bar}
	for i := m.top; len(rows) < h; i++ {
		if i >= len(m.nodes) {
			rows = append(rows, strings.Repeat(" ", inner)+bar)
			continue
		}
		n := m.nodes[i]
		icon := "  "
		if n.Dir {
			icon = "▸ "
			if m.expanded[n.Path] {
				icon = "▾ "
			}
		}
		text := fit(" "+strings.Repeat("  ", n.Depth)+icon+n.Label(), inner)
		rows = append(rows, th.Style(m.slot(i, n, focused, open)).Render(text)+bar)
	}
	return strings.Join(rows, "\n")
}

func (m Model) slot(i int, n Node, focused bool, open string) theme.Slot {
	switch {
	case i == m.sel && focused:
		return theme.SidebarSelected
	case i == m.sel:
		return theme.SidebarSelectedBlur
	case n.Path == open:
		return theme.SidebarOpenFile
	case n.Hidden:
		return theme.SidebarHidden
	case n.Dir:
		return theme.SidebarDir
	case !n.Note():
		return theme.SidebarOtherFile
	}
	return theme.SidebarFile
}

// fit truncates s to w cells with an ellipsis and pads it to w.
func fit(s string, w int) string {
	s = ansi.Truncate(s, w, "…")
	return s + strings.Repeat(" ", max(0, w-ansi.StringWidth(s)))
}
