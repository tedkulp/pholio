package app

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/tedkulp/pholio/internal/theme"
)

// whichKeyDelay is how long spc waits for the next key before the
// which-key popup lists the Leader keys. Fast typists never see it.
const whichKeyDelay = 300 * time.Millisecond

// leaderTickMsg opens the which-key popup for the spc pressed gen times
// in, unless that Leader has already ended.
type leaderTickMsg struct{ gen int }

// startLeader (spc) waits for a Leader key, and schedules the which-key
// popup in case none comes soon.
func (m Model) startLeader() (Model, tea.Cmd) {
	m.leader, m.whichKey = true, false
	m.leaderGen++
	gen := m.leaderGen
	return m, tea.Tick(whichKeyDelay, func(time.Time) tea.Msg { return leaderTickMsg{gen} })
}

// leaderTick shows the which-key popup if the spc it was scheduled for is
// still waiting.
func (m Model) leaderTick(msg leaderTickMsg) Model {
	if m.leader && msg.gen == m.leaderGen {
		m.whichKey = true
	}
	return m
}

// whichKeyShown reports whether the which-key popup is drawn.
func (m Model) whichKeyShown() bool { return m.leader && m.whichKey }

// whichKeyEntry is one "key → help" entry.
func whichKeyEntry(b binding) string { return b.key + " → " + b.help }

// drawWhichKey lays the which-key popup over the bottom of the body: a
// rule, then every Leader key in columns that wrap to fit the width,
// filled top to bottom. On a short screen it keeps as many rows as fit.
// The cursor is dropped when the popup covers it.
func (m Model) drawWhichKey(th theme.Theme, body string, cursor *tea.Cursor) (string, *tea.Cursor) {
	lines := strings.Split(body, "\n")
	var entries []string
	colW := 0
	for _, b := range bindings {
		if b.scope == scopeLeader {
			e := whichKeyEntry(b)
			entries = append(entries, e)
			colW = max(colW, ansi.StringWidth(e))
		}
	}
	colW += 2 // the gap between columns
	cols := max(1, (m.w-1)/colW)
	n := (len(entries) + cols - 1) / cols
	popup := []string{th.Style(theme.OverlayBorder).Render(strings.Repeat("─", m.w))}
	key, hint, box := th.Style(theme.OverlayTitle), th.Style(theme.OverlayHint), th.Style(theme.OverlayBox)
	for r := range n {
		row, used := box.Render(" "), 1
		for c := range cols {
			i := c*n + r
			if i >= len(entries) {
				break
			}
			k, help, _ := strings.Cut(entries[i], " → ")
			pad := colW - ansi.StringWidth(entries[i])
			row += key.Render(k) + hint.Render(" → ") + box.Render(help+strings.Repeat(" ", pad))
			used += colW
		}
		row = ansi.Truncate(row, m.w, "") + box.Render(strings.Repeat(" ", max(0, m.w-used)))
		popup = append(popup, row)
	}
	if len(popup) > len(lines) {
		popup = popup[:len(lines)]
	}
	top := len(lines) - len(popup)
	copy(lines[top:], popup)
	if cursor != nil && cursor.Y >= top {
		cursor = nil
	}
	return strings.Join(lines, "\n"), cursor
}
