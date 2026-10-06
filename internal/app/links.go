package app

import (
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/tedkulp/pholio/internal/engine"
	"github.com/tedkulp/pholio/internal/index"
)

// urlPattern finds http(s) URLs, bare or as a Markdown link's target.
var urlPattern = regexp.MustCompile(`https?://[^\s<>()\[\]"'` + "`" + `]+`)

// followOrMove (enter) follows the Link under the cursor, or else moves
// down a line as vim's enter does.
func (m Model) followOrMove() (Model, tea.Cmd) {
	if m, cmd, ok := m.followLink(); ok {
		return m, cmd
	}
	return m.editorKey(tea.KeyPressMsg{Code: tea.KeyEnter})
}

// goToLink (gd) follows the Link under the cursor.
func (m Model) goToLink() (Model, tea.Cmd) {
	if m, cmd, ok := m.followLink(); ok {
		return m, cmd
	}
	return m.say("No Link under the cursor", false), nil
}

// followLink follows the Link or URL under the cursor. ok is false when
// there is none.
func (m Model) followLink() (Model, tea.Cmd, bool) {
	e := m.ed.Engine()
	line := e.Buf.Line(e.Cur.Line)
	if url, ok := urlAt(line, e.Cur.Col); ok {
		return m.openURL(url), nil, true
	}
	l, ok := m.linkAt(e)
	if !ok {
		return m, nil, false
	}
	m, cmd := m.follow(l)
	return m, cmd, true
}

// urlAt finds the URL that covers byte col of line.
func urlAt(line string, col int) (string, bool) {
	for _, r := range urlPattern.FindAllStringIndex(line, -1) {
		url := strings.TrimRight(line[r[0]:r[1]], ".,;:!?")
		// The text of [text](url) belongs to the URL too.
		start := r[0]
		if strings.HasSuffix(line[:start], "](") {
			if open := strings.LastIndexByte(line[:start-2], '['); open >= 0 {
				start = open
			}
		}
		end := r[0] + len(url)
		if end < len(line) && line[end] == ')' && start < r[0] {
			end++
		}
		if start <= col && col < end {
			return url, true
		}
	}
	return "", false
}

// openURL hands url to the Opener.
func (m Model) openURL(url string) Model {
	if m.deps.Opener == nil {
		return m.say("Can't open URLs here: "+url, true)
	}
	if err := m.deps.Opener.Open(url); err != nil {
		return m.say("opening "+url+": "+err.Error(), true)
	}
	return m.say("Opened "+url, false)
}

// linkAt finds the Link under the cursor. The whole buffer is parsed so
// that Links in fenced code don't count.
func (m Model) linkAt(e *engine.Engine) (index.Link, bool) {
	n := index.Parse(m.rel(m.path()), []byte(e.Buf.String()))
	for _, l := range n.Links {
		if l.Line == e.Cur.Line && l.Start <= e.Cur.Col && e.Cur.Col < l.End {
			return l, true
		}
	}
	return index.Link{}, false
}

// follow opens the Note a Link points to, at its heading if it names one.
// A Link to a missing Note opens an unsaved buffer for it.
func (m Model) follow(l index.Link) (Model, tea.Cmd) {
	from := m.rel(m.path())
	var target, warning string
	switch {
	case l.Target == "":
		target = m.path()
	case l.Kind == index.MarkdownLink:
		p := path.Join(path.Dir(from), l.Target)
		if !inVault(p) {
			return m.say("Link leaves the Vault: "+l.Target, true), nil
		}
		target = m.abs(p)
	default:
		if !m.indexReady() {
			return m.say("indexing…", false), nil
		}
		r, ok := m.index().Resolve(from, l)
		if !ok {
			return m.dangling(l.Target)
		}
		target = m.abs(r.Path)
		if r.Ambiguous {
			warning = fmt.Sprintf("[[%s]] is ambiguous: %s", l.Target, strings.Join(r.Candidates, ", "))
		}
	}
	return m.openAt(target, func(m Model) Model {
		if l.Heading != "" {
			m = m.toHeading(l.Heading)
		}
		if warning != "" {
			m = m.say(warning, true)
		}
		return m
	})
}

// dangling opens an unsaved buffer for <new_note_folder>/<target>.md, the
// Note a dangling Link names. Nothing is written until :w.
func (m Model) dangling(target string) (Model, tea.Cmd) {
	folder := ""
	if m.session != nil {
		folder = m.session.Config.NewNoteFolder
	}
	p := path.Join(folder, index.TrimNoteExt(target)+".md")
	if !inVault(p) {
		return m.say("Link leaves the Vault: "+target, true), nil
	}
	return m.openAt(m.abs(p), func(m Model) Model {
		return m.say("New Note "+p+": :w to create it", false)
	})
}

// toHeading moves the cursor to the open Note's heading named text, ignoring
// case.
func (m Model) toHeading(text string) Model {
	e := m.ed.Engine()
	for _, h := range index.Parse(m.rel(m.path()), []byte(e.Buf.String())).Headings {
		if strings.EqualFold(h.Text, text) {
			e.SetCursor(engine.Pos{Line: h.Line})
			return m
		}
	}
	return m.say("No heading #"+text+" in "+m.rel(m.path()), true)
}

// abs turns a Vault-relative slash path into an OS path.
func (m Model) abs(rel string) string { return filepath.Join(m.vault, filepath.FromSlash(rel)) }

// inVault reports whether the cleaned Vault-relative path p stays inside
// the Vault.
func inVault(p string) bool {
	return p != ".." && !strings.HasPrefix(p, "../") && !path.IsAbs(p)
}
