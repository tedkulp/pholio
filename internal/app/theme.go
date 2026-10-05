package app

import (
	"strings"

	"github.com/tedkulp/pholio/internal/config"
	"github.com/tedkulp/pholio/internal/sidebar"
	"github.com/tedkulp/pholio/internal/theme"
)

// looks is the theme in use and how it was chosen.
type looks struct {
	theme theme.Theme
	// name is the theme asked for (by config or F7). It can differ from
	// theme.Name() when that theme was missing and default was used.
	name string
	// configured is the config's theme setting when it was last read, so
	// F8 can tell whether it changed.
	configured string
}

func defaultLooks() looks {
	return looks{theme: theme.Default(), name: theme.DefaultName, configured: theme.DefaultName}
}

// WithSession applies the startup config: it loads the configured theme and
// shows any config or theme problems on the status line.
func (m Model) WithSession(s config.Session) Model {
	m.session = &s
	problems := m.loadTheme(s.Config.Theme)
	m.looks.configured = s.Config.Theme
	m.wrap, m.conceal = s.Config.Wrap, s.Config.Conceal
	m.vault = s.Target.Vault
	m.ed = m.newEditor(m.ed.Engine())
	m.side = sidebar.New(m.deps.FS, m.vault).Reveal(m.path())
	store := config.NewStateStore(m.deps.FS, s.Dirs)
	m.state = &store
	st, err := store.Load()
	m.sideW = st.SidebarWidth
	stateMsg := ""
	if err != nil {
		stateMsg = "state: " + err.Error()
	}
	m.message = joinMessages(s.Message, theme.Message(problems), stateMsg)
	m.problem = m.message != ""
	return m.relayout()
}

func (m Model) themeDir() string {
	if m.session == nil {
		return ""
	}
	return theme.Dir(m.session.Dirs.ConfigHome)
}

// loadTheme switches to the named theme and returns its problems.
func (m *Model) loadTheme(name string) []string {
	th, problems := theme.Load(m.deps.FS, m.themeDir(), name)
	m.looks.theme, m.looks.name = th, name
	return problems
}

// cycleTheme (F7) moves to the next available theme.
func (m Model) cycleTheme() Model {
	names := theme.Names(m.deps.FS, m.themeDir())
	next := names[0]
	for i, n := range names {
		if n == m.looks.name {
			next = names[(i+1)%len(names)]
			break
		}
	}
	m.message = theme.Message(m.loadTheme(next))
	m.problem = m.message != ""
	if !m.problem {
		m.message = "theme: " + next
	}
	return m
}

// reload (F8) rereads both config files and the theme. A theme picked with
// F7 is kept unless the config's theme setting changed.
func (m Model) reload() Model {
	name, configMsg := m.looks.name, ""
	if m.session != nil {
		s := *m.session // copy: earlier Model values share the old pointer
		cfg, msg := s.Reload(m.deps.FS)
		s.Config, configMsg = cfg, msg
		m.session = &s
		m.wrap, m.conceal = cfg.Wrap, cfg.Conceal
		m.ed = m.ed.SetWrap(cfg.Wrap).SetConceal(cfg.Conceal)
		if cfg.Theme != m.looks.configured {
			name, m.looks.configured = cfg.Theme, cfg.Theme
		}
	}
	m.message = joinMessages(configMsg, theme.Message(m.loadTheme(name)))
	m.problem = m.message != ""
	if !m.problem {
		m.message = "reloaded config and theme"
	}
	return m
}

func joinMessages(msgs ...string) string {
	var parts []string
	for _, s := range msgs {
		if s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, "  ")
}
