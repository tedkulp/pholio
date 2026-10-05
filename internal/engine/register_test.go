package engine

import (
	"errors"
	"testing"
)

func TestRegisteredExCommand(t *testing.T) {
	cases := []struct {
		keys string
		want ExCmd
	}{
		{":tasks<enter>", ExCmd{Name: "tasks"}},
		{":grep  foo bar <enter>", ExCmd{Name: "grep", Arg: "foo bar"}},
		{":delete!<enter>", ExCmd{Name: "delete", Bang: true}},
	}
	for _, c := range cases {
		e := load("|abc")
		var got []ExCmd
		run := func(e2 *Engine, cmd ExCmd) error {
			if e2 != e {
				t.Error("handler got a different engine")
			}
			got = append(got, cmd)
			return nil
		}
		e.Register("tasks", run)
		e.Register("grep", run)
		e.Register("delete", run)
		feed(e, c.keys)
		if len(got) != 1 || got[0] != c.want {
			t.Errorf("%q: handler got %+v, want %+v", c.keys, got, c.want)
		}
		if e.Msg != "" || e.Mode != Normal {
			t.Errorf("%q: msg %q mode %v", c.keys, e.Msg, e.Mode)
		}
	}
}

func TestRegisteredExCommandError(t *testing.T) {
	e := load("|abc")
	e.Register("today", func(*Engine, ExCmd) error { return errors.New("no vault") })
	feed(e, ":today<enter>")
	if e.Msg != "no vault" {
		t.Errorf("msg = %q", e.Msg)
	}
}

func TestRegisteredExCommandCanEdit(t *testing.T) {
	// A handler may change the buffer through the engine's normal API, and
	// that change is one undo step.
	e := load("|abc")
	e.Register("chop", func(e *Engine, _ ExCmd) error {
		e.Feed("x")
		e.Feed("x")
		return nil
	})
	feed(e, ":chop<enter>")
	if g := show(e); g != "|c" {
		t.Errorf("after :chop %q", g)
	}
	feed(e, "u")
	if g := show(e); g != "|abc" {
		t.Errorf("after u %q", g)
	}
}

func TestBuiltinsWinOverRegistered(t *testing.T) {
	e := load("|abc")
	called := false
	e.Register("noh", func(*Engine, ExCmd) error { called = true; return nil })
	feed(e, ":noh<enter>:zzz<enter>")
	if called {
		t.Error("registered noh ran instead of the built-in")
	}
	if e.Msg != "E492: Not an editor command: zzz" {
		t.Errorf("msg = %q", e.Msg)
	}
}
