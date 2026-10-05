package app

import (
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"

	"github.com/tedkulp/pholio/internal/engine"
	"github.com/tedkulp/pholio/internal/index"
	"github.com/tedkulp/pholio/internal/palette"
)

// grepLimit is the most results Vault search lists.
const grepLimit = 200

// grepDebounce is how long the query has to stay unchanged before Vault
// search runs it.
const grepDebounce = 120 * time.Millisecond

// match finds query in line: a literal, smart-case substring search (a
// query with no capitals ignores case, as the engine's / does). It returns
// the [start, end) byte ranges of the matches, left to right. It is the
// one matcher behind Vault search, so regex can be added here later.
func match(query, line string) [][2]int {
	if query == "" {
		return nil
	}
	hay := line
	switch {
	case strings.ToLower(query) != query: // a capital: case matters
	case isASCII(query):
		// Lowering only ASCII keeps every byte offset of line.
		hay = asciiLower(line)
	default:
		return foldMatches(query, line)
	}
	var out [][2]int
	for at := 0; ; {
		i := strings.Index(hay[at:], query)
		if i < 0 {
			return out
		}
		at += i
		out = append(out, [2]int{at, at + len(query)})
		at += len(query)
	}
}

func isASCII(s string) bool {
	for i := range len(s) {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

func asciiLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}

// foldMatches is match ignoring case for a query with non-ASCII letters,
// comparing rune by rune so offsets stay those of line.
func foldMatches(query, line string) [][2]int {
	var out [][2]int
	for i := 0; i < len(line); {
		if n, ok := foldPrefix(line[i:], query); ok {
			out = append(out, [2]int{i, i + n})
			i += n
			continue
		}
		_, size := utf8.DecodeRuneInString(line[i:])
		i += size
	}
	return out
}

// foldPrefix reports whether s starts with query, ignoring case, and how
// many bytes of s that covers.
func foldPrefix(s, query string) (int, bool) {
	n := 0
	for _, q := range query {
		if n >= len(s) {
			return 0, false
		}
		r, size := utf8.DecodeRuneInString(s[n:])
		if r != q && unicode.ToLower(r) != unicode.ToLower(q) && unicode.ToUpper(r) != unicode.ToUpper(q) {
			return 0, false
		}
		n += size
	}
	return n, true
}

// grepHit is one line that matches a Vault search.
type grepHit struct {
	path  string // Vault-relative
	line  int    // 0-based
	col   int    // byte offset of the first match
	text  string // the line
	marks [][2]int
	query string
}

// grep searches every Note in ix for query, in path then line order. more
// is set when it stopped at the limit. It is safe off the update loop.
func grep(ix *index.Index, query string) (hits []grepHit, more bool) {
	for _, n := range ix.Notes() {
		if match(query, n.Contents) == nil { // most Notes: skip the split
			continue
		}
		for i, line := range strings.Split(n.Contents, "\n") {
			marks := match(query, line)
			if marks == nil {
				continue
			}
			if len(hits) == grepLimit {
				return hits, true
			}
			hits = append(hits, grepHit{path: n.Path, line: i, col: marks[0][0], text: line, marks: marks, query: query})
		}
	}
	return hits, false
}

// snippet is the hit's line for a result row: leading space trimmed, tabs
// as spaces, and a long lead-in before the first match cut to "…".
func (h grepHit) snippet() (string, [][2]int) {
	text := strings.ReplaceAll(h.text, "\t", " ")
	cut := len(text) - len(strings.TrimLeft(text, " "))
	if lead := h.marks[0][0] - cut; lead > 30 {
		at := h.marks[0][0] - 15
		for at > 0 && !utf8.RuneStart(text[at]) {
			at--
		}
		cut = at
	}
	prefix := ""
	if cut > 0 && strings.TrimSpace(text[:cut]) != "" {
		prefix = "…"
	}
	shift := len(prefix) - cut
	marks := make([][2]int, len(h.marks))
	for i, mk := range h.marks {
		marks[i] = [2]int{mk[0] + shift, mk[1] + shift}
	}
	return prefix + text[cut:], marks
}

// grepState is shared by an open Vault search and its debounce ticks.
type grepState struct {
	gen  int  // the newest query's tick
	open bool // the search palette is still up
}

// grepTickMsg runs the query typed gen keys ago, unless a newer one came.
type grepTickMsg struct {
	st  *grepState
	gen int
}

// showGrep (spc /, :grep [query]) opens Vault search, run on query at
// once when it is given.
func (m Model) showGrep(query string) (Model, tea.Cmd) {
	if m.index() == nil {
		return m.say("Search needs the Vault index", true), nil
	}
	m = m.syncIndex() // unsaved edits are searched too
	st := &grepState{open: true}
	every := func(_ string, items []palette.Item) []int {
		out := make([]int, len(items))
		for i := range items {
			out[i] = i
		}
		return out
	}
	p := palette.New("Search the Vault", palette.Type).
		WithPlaceholder("text to find").
		WithMatcher(every).
		WithQuery(query)
	p = m.grepItems(p, query)
	m = m.showPalette(p, func(m Model, ev palette.Event) (Model, tea.Cmd) {
		switch ev.Kind {
		case palette.Changed:
			st.gen++
			gen := st.gen
			return m, tea.Tick(grepDebounce, func(time.Time) tea.Msg { return grepTickMsg{st, gen} })
		case palette.Closed:
			st.open = false
		case palette.Chosen:
			st.open = false
			if ev.OK {
				return m.openHit(ev.Item.Value.(grepHit))
			}
		}
		return m, nil
	})
	m.overlay.ready = func(m Model) Model {
		m.overlay.p = m.grepItems(m.overlay.p, m.overlay.p.Query())
		return m
	}
	return m, nil
}

// grepResultMsg carries the hits of a search run off the update loop.
type grepResultMsg struct {
	st    *grepState
	gen   int
	query string
	hits  []grepHit
	more  bool
}

// grepTick starts a debounced query of the open search palette in the
// background. Its result is dropped if the query changed meanwhile.
func (m Model) grepTick(msg grepTickMsg) (Model, tea.Cmd) {
	if !msg.st.open || msg.gen != msg.st.gen || m.overlay == nil {
		return m, nil
	}
	query := m.overlay.p.Query()
	if query == "" || !m.indexReady() {
		m.overlay.p = m.grepItems(m.overlay.p, query)
		return m, nil
	}
	ix, st, gen := m.index(), msg.st, msg.gen
	return m, func() tea.Msg {
		hits, more := grep(ix, query)
		return grepResultMsg{st: st, gen: gen, query: query, hits: hits, more: more}
	}
}

// grepResult shows a background search's hits, unless they are stale.
func (m Model) grepResult(msg grepResultMsg) Model {
	if !msg.st.open || msg.gen != msg.st.gen || m.overlay == nil || m.overlay.p.Query() != msg.query {
		return m
	}
	m.overlay.p = grepShow(m.overlay.p, msg.hits, msg.more)
	return m
}

const grepKeys = "enter open · esc close"

// grepItems fills p with the results for query, searching now.
func (m Model) grepItems(p palette.Model, query string) palette.Model {
	switch {
	case query == "":
		return p.WithEmpty("type to search the Vault").WithHint(grepKeys).SetItems(nil)
	case !m.indexReady():
		return p.WithEmpty("indexing…").WithHint(grepKeys).SetItems(nil)
	}
	hits, more := grep(m.index(), query)
	return grepShow(p, hits, more)
}

// grepShow fills p with hits.
func grepShow(p palette.Model, hits []grepHit, more bool) palette.Model {
	items := make([]palette.Item, len(hits))
	for i, h := range hits {
		text, marks := h.snippet()
		items[i] = palette.Item{Text: text, Marks: marks, Detail: fmt.Sprintf("%s:%d", h.path, h.line+1), Value: h}
	}
	count := fmt.Sprintf("%d matches", len(hits))
	if more {
		count = fmt.Sprintf("%d+ matches", len(hits))
	}
	return p.WithEmpty("no matches").WithHint(count + " · " + grepKeys).SetItems(items)
}

// openHit opens a search result at its match and loads the query into /,
// so n and N go on from there.
func (m Model) openHit(h grepHit) (Model, tea.Cmd) {
	return m.openAt(m.abs(h.path), func(m Model) Model {
		e := m.ed.Engine()
		e.SetCursor(engine.Pos{Line: h.line, Col: h.col})
		e.SetSearch(h.query)
		return m
	})
}
