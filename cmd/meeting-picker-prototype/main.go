// PROTOTYPE, throw away. Answers wayfinder ticket "How does the Meeting
// picker look and move between days?" (#56): three variants of the picker
// on internal/palette, switchable with tab. Fake data, no feeds, no files.
//
//	go run ./cmd/meeting-picker-prototype
//
// Prototype controls (not part of any design): tab next variant,
// ! toggle "no cache yet" (Loading…), e toggle feed errors, ctrl+c quit.
package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/tedkulp/pholio/internal/palette"
	"github.com/tedkulp/pholio/internal/theme"
)

type meeting struct {
	start, end time.Time
	title, cal string
	noted      bool
}

var today = time.Date(2026, 10, 12, 0, 0, 0, 0, time.Local)
var now = today.Add(11*time.Hour + 10*time.Minute)

// meetingsOn fakes a day of Meetings, sorted by start.
func meetingsOn(day time.Time) []meeting {
	at := func(h, m, mins int, title, cal string, noted bool) meeting {
		s := day.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute)
		return meeting{s, s.Add(time.Duration(mins) * time.Minute), title, cal, noted}
	}
	if day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
		return []meeting{at(10, 0, 60, "Kid's football", "family", false)}
	}
	past := day.Before(today)
	return []meeting{
		at(9, 0, 15, "Daily standup", "work", past || day.Equal(today)),
		at(10, 0, 60, "Architecture review: ICS ingestion", "work", past),
		at(11, 0, 30, "1:1 with Sam", "work", false),
		at(11, 0, 45, "Dentist", "personal", false),
		at(13, 30, 30, "Lunch & learn: Bubble Tea v2", "work-guild", false),
		at(16, 0, 90, "Quarterly planning (very long title that will need truncating somewhere)", "work", false),
	}
}

type variant struct {
	name string
	open func(m model) palette.Model
	// key maps a host key to a day delta ("today" = reset) or refresh.
	key func(k string) (delta int, toToday, refresh, ok bool)
}

type model struct {
	w, h     int
	v        int
	day      time.Time
	loading  bool
	errors   bool
	p        palette.Model
	lastNote string
}

func statusLines(m model) []string {
	if !m.errors {
		return nil
	}
	return []string{"work: offline, showing data from 2h ago", "work-guild: HTTP 401"}
}

func dayLabel(d time.Time) string {
	s := d.Format("Mon 2 Jan")
	switch int(d.Sub(today).Hours() / 24) {
	case 0:
		s += " (today)"
	case -1:
		s += " (yesterday)"
	case 1:
		s += " (tomorrow)"
	}
	return s
}

func span(mt meeting) string { return mt.start.Format("15:04") + "–" + mt.end.Format("15:04") }

// A: flat agenda. List mode, a ✓ gutter for noted Meetings, past Meetings
// dimmed, calendar right-aligned, feed status under the input.
// h/l day, t today, r refresh.
var variantA = variant{
	name: "A flat agenda (h/l day, t today, r refresh)",
	open: func(m model) palette.Model {
		var items []palette.Item
		for _, mt := range meetingsOn(m.day) {
			mark := "  "
			if mt.noted {
				mark = "✓ "
			}
			it := palette.Item{Text: mark + span(mt) + "  " + mt.title, Detail: mt.cal, Value: mt}
			if mt.end.Before(now) {
				it.Slot = theme.OverlayHint
			}
			items = append(items, it)
		}
		p := palette.New("Meetings · "+dayLabel(m.day), palette.List).
			WithPlaceholder("/ to filter").
			WithHint("h/l day · t today · r refresh · enter open/create · esc close").
			WithInfo(func(string) []string { return statusLines(m) })
		if m.loading {
			return p.WithEmpty("Loading…")
		}
		return p.WithEmpty("no Meetings").SetItems(items)
	},
	key: func(k string) (int, bool, bool, bool) {
		switch k {
		case "h", "left":
			return -1, false, false, true
		case "l", "right":
			return 1, false, false, true
		case "t":
			return 0, true, false, true
		case "r":
			return 0, false, true, true
		}
		return 0, false, false, false
	},
}

// B: grouped by calendar. Type mode (filter-first, like Find Note), so day
// keys are non-letters: ←/→ day, ctrl+t today, ctrl+r refresh. "noted" as
// the Detail, feed status folded into the footer.
var variantB = variant{
	name: "B by calendar, type-to-filter (←/→ day, ctrl+t today, ctrl+r refresh)",
	open: func(m model) palette.Model {
		byCal := map[string][]palette.Item{}
		var order []string
		for _, mt := range meetingsOn(m.day) {
			if _, ok := byCal[mt.cal]; !ok {
				order = append(order, mt.cal)
			}
			it := palette.Item{Text: span(mt) + "  " + mt.title, Group: mt.cal, Value: mt}
			if mt.noted {
				it.Detail, it.DetailSlot = "noted", theme.OverlayGroup
			}
			byCal[mt.cal] = append(byCal[mt.cal], it)
		}
		var items []palette.Item
		for _, c := range order {
			items = append(items, byCal[c]...)
		}
		hint := "←/→ day · ctrl+t today · ctrl+r refresh · enter open/create · esc"
		if s := statusLines(m); s != nil {
			hint = "⚠ " + strings.Join(s, " · ")
		}
		p := palette.New("Meetings · "+dayLabel(m.day), palette.Type).
			WithPlaceholder("filter Meetings").WithHint(hint)
		if m.loading {
			return p.WithEmpty("Loading…")
		}
		return p.WithEmpty("no Meetings").SetItems(items)
	},
	key: func(k string) (int, bool, bool, bool) {
		switch k {
		case "left":
			return -1, false, false, true
		case "right":
			return 1, false, false, true
		case "ctrl+t":
			return 0, true, false, true
		case "ctrl+r":
			return 0, false, true, true
		}
		return 0, false, false, false
	},
}

// C: timeline. List mode grouped Earlier / Now / Later, calendar in the
// row, noted as a 📝 detail, selection starts on the next Meeting, feed
// status as its own group at the bottom. [ / ] day (like [d ]d),
// . today, R refresh.
var variantC = variant{
	name: "C timeline, Earlier/Now/Later ([ ] day, . today, R refresh)",
	open: func(m model) palette.Model {
		var items []palette.Item
		first := 0
		for _, mt := range meetingsOn(m.day) {
			g := "Later"
			switch {
			case !m.day.Equal(today):
				g = ""
			case mt.end.Before(now):
				g = "Earlier"
			case mt.start.Before(now):
				g = "Now"
			}
			if g == "Earlier" {
				first = len(items) + 1
			}
			it := palette.Item{Text: span(mt) + "  " + mt.title + "  · " + mt.cal, Group: g, Value: mt}
			if mt.noted {
				it.Detail = "📝"
			}
			items = append(items, it)
		}
		for _, s := range statusLines(m) {
			items = append(items, palette.Item{Text: "⚠ " + s, Group: "Feeds", Slot: theme.UIError})
		}
		p := palette.New("Meetings · "+dayLabel(m.day), palette.List).
			WithHint("[/] day · . today · R refresh · enter open/create · esc close")
		if m.loading {
			return p.WithEmpty("Loading…")
		}
		return p.WithEmpty("no Meetings").SetItems(items).Select(first)
	},
	key: func(k string) (int, bool, bool, bool) {
		switch k {
		case "[":
			return -1, false, false, true
		case "]":
			return 1, false, false, true
		case ".":
			return 0, true, false, true
		case "R":
			return 0, false, true, true
		}
		return 0, false, false, false
	},
}

var variants = []variant{variantA, variantB, variantC}

func (m model) reopen() model { m.p = variants[m.v].open(m); return m }

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
	case tea.KeyPressMsg:
		k := msg.Keystroke()
		if msg.Text != "" && msg.Mod&(tea.ModCtrl|tea.ModAlt) == 0 {
			k = msg.Text
		}
		switch k {
		case "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.v = (m.v + 1) % len(variants)
			return m.reopen(), nil
		case "!":
			m.loading = !m.loading
			return m.reopen(), nil
		case "e":
			if m.v != 1 || !m.p.Typing() {
				m.errors = !m.errors
				return m.reopen(), nil
			}
		}
		if d, toToday, refresh, ok := variants[m.v].key(k); ok && !(m.p.Typing() && len(k) == 1) {
			switch {
			case toToday:
				m.day = today
			case refresh:
				m.lastNote = "refreshing all feeds…"
			default:
				m.day = m.day.AddDate(0, 0, d)
			}
			return m.reopen(), nil
		}
		var ev palette.Event
		m.p, ev = m.p.Update(msg)
		switch ev.Kind {
		case palette.Closed:
			return m, tea.Quit
		case palette.Chosen:
			if mt, ok := ev.Item.Value.(meeting); ok {
				verb := "create"
				if mt.noted {
					verb = "open"
				}
				m.lastNote = fmt.Sprintf("would %s meetings/%s-%s.md", verb, mt.start.Format("2006-01-02-1504"), "…")
			}
		}
	}
	return m, nil
}

func (m model) View() tea.View {
	var v tea.View
	v.AltScreen = true
	if m.w == 0 {
		return v
	}
	th := theme.Default()
	box, x, y, cursor := m.p.View(th, m.w, m.h)
	lines := make([]string, m.h)
	for i, l := range strings.Split(box, "\n") {
		if y+i < m.h-2 {
			lines[y+i] = strings.Repeat(" ", x) + l
		}
	}
	lines[m.h-2] = m.lastNote
	lines[m.h-1] = th.Style(theme.UIModeCommand).Render(fmt.Sprintf(" PROTOTYPE %d/%d ", m.v+1, len(variants))) +
		" " + variants[m.v].name + "   [tab variant · ! loading · e errors]"
	v.SetContent(strings.Join(lines, "\n"))
	v.Cursor = cursor
	return v
}

func main() {
	m := model{day: today}.reopen()
	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
