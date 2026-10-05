package app

import (
	"slices"

	tea "charm.land/bubbletea/v2"

	"github.com/tedkulp/pholio/internal/engine"
)

// jump is a place in a Note.
type jump struct {
	path string
	pos  engine.Pos
}

// jumplist is the session's Note history: ctrl+o goes back, tab/ctrl+i
// forward. While going back and forth, list[i] is the current place; after
// a new jump, i is len(list) and the current place is not in the list.
type jumplist struct {
	list []jump
	i    int
}

// push records from, the place being left by a new jump. Any forward
// history is dropped.
func (j jumplist) push(from jump) jumplist {
	list := append(slices.Clone(j.list[:j.i]), from)
	return jumplist{list: list, i: len(list)}
}

// back steps back from cur. ok is false at the oldest entry.
func (j jumplist) back(cur jump) (jumplist, jump, bool) {
	list := slices.Clone(j.list)
	if j.i == len(list) {
		list = append(list, cur)
	} else {
		list[j.i] = cur
	}
	if j.i == 0 {
		return j, jump{}, false
	}
	return jumplist{list: list, i: j.i - 1}, list[j.i-1], true
}

// forward steps forward from cur. ok is false at the newest entry.
func (j jumplist) forward(cur jump) (jumplist, jump, bool) {
	if j.i >= len(j.list)-1 {
		return j, jump{}, false
	}
	list := slices.Clone(j.list)
	list[j.i] = cur
	return jumplist{list: list, i: j.i + 1}, list[j.i+1], true
}

// here is the current place.
func (m Model) here() jump { return jump{m.path(), m.ed.Engine().Cur} }

// jumpBack (ctrl+o) returns to the previous place in the jumplist.
func (m Model) jumpBack() (Model, tea.Cmd) {
	j, to, ok := m.jumps.back(m.here())
	if !ok {
		return m.say("At the start of the jumplist", false), nil
	}
	return m.goJump(j, to)
}

// jumpForward (tab, ctrl+i) undoes a jumpBack.
func (m Model) jumpForward() (Model, tea.Cmd) {
	j, to, ok := m.jumps.forward(m.here())
	if !ok {
		return m.say("At the end of the jumplist", false), nil
	}
	return m.goJump(j, to)
}

// goJump moves to a jumplist entry without recording a new one. The
// jumplist only moves once the switch happens, so cancelling the dirty
// buffer prompt leaves it as it was.
func (m Model) goJump(j jumplist, to jump) (Model, tea.Cmd) {
	arrive := func(m Model) Model {
		m.jumps = j
		m.ed.Engine().SetCursor(to.pos)
		return m
	}
	if to.path == m.path() {
		return arrive(m), nil
	}
	return m.unlessDirty(saveBeforeSwitch, func(m Model) (Model, tea.Cmd) {
		m, ok := m.switchTo(to.path)
		if !ok {
			return m, nil
		}
		return arrive(m), nil
	})
}
