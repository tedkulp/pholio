package app

import (
	"cmp"
	"net/http"
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/tedkulp/pholio/internal/calendar"
	"github.com/tedkulp/pholio/internal/config"
	"github.com/tedkulp/pholio/internal/palette"
)

// meetingState is shared by an open Meeting picker and its feed fetches.
type meetingState struct {
	open    bool      // the picker is still up
	day     time.Time // the day shown
	pending int       // feeds still being fetched
	rows    []meetingRow
	failed  []string          // "name: why", one per failing calendar
	notes   map[string]string // meeting_uid → Meeting Note path
}

// meetingRow is a Meeting and the name of the calendar it came from.
type meetingRow struct {
	calendar.Meeting
	cal string
}

// meetingsMsg is one calendar's fetch result.
type meetingsMsg struct {
	st  *meetingState
	cal string
	ms  []calendar.Meeting
	err error
}

// showMeetings (spc m, :meetings) opens the Meeting picker on Today and
// fetches every calendar, one goroutine each.
func (m Model) showMeetings() (Model, tea.Cmd) {
	cals := m.config().Calendars
	if len(cals) == 0 {
		where := "the user config"
		if m.session != nil {
			where = config.UserConfigPath(m.session.Dirs)
		}
		return m.say("no calendars: add [[calendars]] to "+where, true), nil
	}
	day := m.today()
	st := &meetingState{open: true, day: day, pending: len(cals), notes: m.meetingNotes()}
	p := palette.New("Meetings · "+day.Format("Mon 2 Jan")+" (today)", palette.List).
		WithPlaceholder("/ to filter").
		WithHint("/ filter · esc close").
		WithInfo(func(string) []string { return st.failed })
	m = m.showPalette(meetingItems(p, st), func(m Model, ev palette.Event) (Model, tea.Cmd) {
		if ev.Kind == palette.Closed || ev.Kind == palette.Chosen {
			st.open = false
		}
		if r, ok := ev.Item.Value.(meetingRow); ok && ev.Kind == palette.Chosen {
			return m.openMeeting(r.Meeting)
		}
		return m, nil
	})
	client := &http.Client{Timeout: calendar.Timeout}
	cmds := make([]tea.Cmd, len(cals))
	for i, c := range cals {
		cmds[i] = func() tea.Msg {
			ms, err := calendar.Fetch(client, c.URL)
			return meetingsMsg{st: st, cal: c.Name, ms: ms, err: err}
		}
	}
	return m, tea.Batch(cmds...)
}

// meetingsArrived merges one calendar's Meetings into the open picker.
func (m Model) meetingsArrived(msg meetingsMsg) Model {
	st := msg.st
	if !st.open || m.overlay == nil {
		return m
	}
	st.pending--
	if msg.err != nil {
		st.failed = append(st.failed, msg.cal+": "+msg.err.Error())
	}
	for _, mt := range calendar.On(msg.ms, st.day) {
		st.rows = append(st.rows, meetingRow{mt, msg.cal})
	}
	m.overlay.p = meetingItems(m.overlay.p, st)
	return m
}

// meetingItems fills p with st's Meetings, sorted by start.
func meetingItems(p palette.Model, st *meetingState) palette.Model {
	slices.SortStableFunc(st.rows, func(a, b meetingRow) int { return cmp.Compare(a.Start.UnixNano(), b.Start.UnixNano()) })
	loc := st.day.Location()
	items := make([]palette.Item, len(st.rows))
	for i, r := range st.rows {
		gutter := "  "
		if _, ok := st.notes[meetingUID(r.Meeting)]; ok {
			gutter = "✓ "
		}
		items[i] = palette.Item{
			Text:   gutter + r.Start.In(loc).Format("15:04") + "–" + r.End.In(loc).Format("15:04") + "  " + r.Title,
			Detail: r.cal,
			Value:  r,
		}
	}
	empty := "no Meetings"
	if st.pending > 0 {
		empty = "Loading…"
	}
	return p.WithEmpty(empty).SetItems(items)
}
