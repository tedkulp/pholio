package app

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/tedkulp/pholio/internal/config"
	"github.com/tedkulp/pholio/internal/daily"
	"github.com/tedkulp/pholio/internal/dates"
	"github.com/tedkulp/pholio/internal/palette"
)

// Start opens what startup asked for: the file argument, or else today's
// Daily Note (created from the Template if it is missing), or, with
// open_daily_on_startup = false, an empty unnamed buffer with the sidebar
// focused.
func Start(deps Deps, s config.Session) (Model, error) {
	path := s.Target.File
	if path == "" && s.Config.OpenDailyOnStartup {
		m := Model{deps: deps, session: &s}
		p, err := m.ensureDaily(m.today())
		if err != nil {
			return Model{}, err
		}
		path = p
	}
	m, err := New(deps, path)
	if err != nil {
		return Model{}, err
	}
	m = m.WithSession(s)
	if path == "" {
		m = m.focusSidebar()
	}
	return m, nil
}

// config is the session's config, or the defaults without a session.
func (m Model) config() config.Config {
	if m.session == nil {
		return config.Default()
	}
	return m.session.Config
}

// now is the injected clock's time.
func (m Model) now() time.Time {
	if m.deps.Clock == nil {
		return time.Now()
	}
	return m.deps.Clock.Now()
}

// today is Today: the day pholio counts as today, shifted by
// day_starts_at. Every feature that needs "today" asks here.
func (m Model) today() time.Time {
	return dates.Today(m.now(), m.config().DayStartsAt)
}

// dailyNotes are the Vault's Daily Notes, as configured.
func (m Model) dailyNotes() daily.Notes {
	cfg := m.config()
	vault := m.vault
	if m.session != nil {
		vault = m.session.Target.Vault
	}
	return daily.Notes{FS: m.deps.FS, Vault: vault, Folder: cfg.DailyFolder, Template: cfg.DailyTemplate}
}

// ensureDaily creates day's Daily Note if it is missing and returns its
// path. pholio's own write is not reported back as an outside change.
func (m Model) ensureDaily(day time.Time) (string, error) {
	path, data, err := m.dailyNotes().Ensure(day, m.now())
	if err != nil {
		return "", fmt.Errorf("creating Daily Note %s: %w", day.Format(dates.ISO), err)
	}
	if data != nil && m.deps.Watch != nil {
		m.deps.Watch.Wrote(path, data)
	}
	return path, nil
}

// openDaily (spc d, :today, :daily) opens day's Daily Note, creating it
// from the Template once the open buffer has been dealt with.
func (m Model) openDaily(day time.Time) (Model, tea.Cmd) {
	path := m.dailyNotes().Path(day)
	if path == m.path() {
		m.focus = focusEditor
		return m, nil
	}
	return m.unlessDirty(saveBeforeSwitch, func(m Model) (Model, tea.Cmd) {
		if _, err := m.ensureDaily(day); err != nil {
			return m.say(err.Error(), true), nil
		}
		return m.switchTo(path), nil
	})
}

// stepDaily ([d, ]d) moves to the nearest existing Daily Note before
// (dir < 0) or after the open one, or Today when the open Note is not a
// Daily Note. It never creates one.
func (m Model) stepDaily(dir int) (Model, tea.Cmd) {
	n := m.dailyNotes()
	from, ok := n.Day(m.path())
	if !ok {
		from = m.today()
	}
	step, word := n.Next, "later"
	if dir < 0 {
		step, word = n.Prev, "earlier"
	}
	day, ok, err := step(from)
	switch {
	case err != nil:
		return m.say("listing Daily Notes: "+err.Error(), true), nil
	case !ok:
		return m.say("no "+word+" Daily Note", false), nil
	}
	return m.openNote(n.Path(day))
}

// openDate (:daily <date>) opens the Daily Note for an ISO or relative
// date, creating it if it is missing.
func (m Model) openDate(s string) (Model, tea.Cmd) {
	day, ok := dates.Parse(s, m.today())
	if !ok {
		return m.say(fmt.Sprintf("not a date: %q", strings.TrimSpace(s)), true), nil
	}
	return m.openDaily(day)
}

// jumpToDate (spc D, :daily) asks for a date, previewing what it means,
// then opens that day's Daily Note. With an argument it skips the prompt.
func (m Model) jumpToDate(arg string) (Model, tea.Cmd) {
	if strings.TrimSpace(arg) != "" {
		return m.openDate(arg)
	}
	today, n := m.today(), m.dailyNotes()
	p := palette.New("Jump to date", palette.Type).
		WithPlaceholder("2026-10-05, today, friday, +3d, -1w").
		WithHint("enter open · esc close").
		WithInfo(func(q string) []string {
			if strings.TrimSpace(q) == "" {
				return nil
			}
			day, ok := dates.Parse(q, today)
			if !ok {
				return []string{"not a date"}
			}
			line := day.Format("Monday " + dates.ISO)
			if _, err := m.deps.FS.Stat(n.Path(day)); err != nil {
				line += " · new"
			}
			return []string{line}
		})
	return m.showPalette(p, func(m Model, ev palette.Event) (Model, tea.Cmd) {
		if ev.Kind != palette.Chosen || strings.TrimSpace(ev.Query) == "" {
			return m, nil
		}
		return m.openDate(ev.Query)
	}), nil
}
