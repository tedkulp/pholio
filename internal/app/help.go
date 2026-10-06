package app

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/tedkulp/pholio/internal/palette"
)

// helpScopes orders the help popup's rows and names each scope.
var helpScopes = []struct {
	scope scope
	name  string
}{
	{scopeLeader, "leader"},
	{scopeGlobal, "global"},
	{scopeSequence, "sequence"},
	{scopeNormal, "normal"},
	{scopeSidebar, "sidebar"},
	{scopeApp, "app"},
}

// helpKeysW is the width of the keys column.
const helpKeysW = 10

// showHelp (spc ?) opens the help popup: one row per binding, which runs
// it on enter, so it doubles as a command palette.
func (m Model) showHelp() Model {
	var items []palette.Item
	for _, s := range helpScopes {
		for _, b := range bindings {
			if b.scope == s.scope {
				items = append(items, palette.Item{
					Text:   fmt.Sprintf("%-*s%s", helpKeysW, typedKeys(b), b.help),
					Detail: s.name,
					Value:  b,
				})
			}
		}
	}
	p := palette.New("Help", palette.Type).WithPlaceholder("filter keys").SetItems(items)
	return m.showPalette(p, onHelp)
}

// typedKeys is a binding's keys the way they are typed: "spc d", "F7".
func typedKeys(b binding) string {
	k := b.key
	switch {
	case k == " ":
		k = "spc"
	case len(k) > 1 && k[0] == 'f' && strings.Trim(k[1:], "0123456789") == "":
		k = "F" + k[1:]
	}
	if b.scope == scopeLeader {
		k = "spc " + k
	}
	return k
}

func onHelp(m Model, ev palette.Event) (Model, tea.Cmd) {
	b, ok := ev.Item.Value.(binding)
	if ev.Kind != palette.Chosen || !ev.OK || !ok {
		return m, nil
	}
	switch {
	case b.action == actLeader || b.action == actHelp:
		return m, nil
	case b.scope == scopeSidebar && !m.sidebarFocused():
		return m.say("focus the sidebar first", false), nil
	}
	return m.run(b.action, "")
}
