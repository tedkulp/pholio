package app_test

import (
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/tedkulp/pholio/internal/app"
	"github.com/tedkulp/pholio/internal/engine"
	"github.com/tedkulp/pholio/internal/seam/seamtest"
	"github.com/tedkulp/pholio/internal/testutil"
)

func TestGrepWaitsForTheIndex(t *testing.T) {
	v := linksVault{root: testutil.CopyVault(t, "links"), opener: &seamtest.Opener{}}
	m := resize(openIn(t, &v, "index.md"), 100, 30)
	scan := m.Init()

	m = ex(m, "grep points")
	if s := screen(m); !strings.Contains(s, "indexing…") {
		t.Fatalf("no indexing notice before the scan:\n%s", s)
	}
	m = drive(t, m, scan)
	if s := screen(m); !strings.Contains(s, "alpha.md:5") {
		t.Errorf("results did not arrive with the index:\n%s", s)
	}
}

// typeDriven types s, running each key's commands (the search debounce)
// as the program would.
func typeDriven(t *testing.T, m app.Model, s string) app.Model {
	t.Helper()
	for _, r := range s {
		next, cmd := m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = drive(t, next.(app.Model), cmd)
	}
	return m
}

func TestGrepIsCaseInsensitiveForALowercaseQuery(t *testing.T) {
	m, _ := linked(t, "index.md")
	m = resize(m, 100, 30)

	m = ex(m, "grep alpha")

	s := screen(m)
	for _, want := range []string{"alpha.md:1", "alpha.md:5", "index.md:3", "sub/gamma.md:3"} {
		if !strings.Contains(s, want) {
			t.Errorf("screen lacks %q:\n%s", want, s)
		}
	}
}

func TestGrepIsCaseSensitiveWhenTheQueryHasACapital(t *testing.T) {
	m, _ := linked(t, "index.md")
	m = resize(m, 100, 30)

	m = ex(m, "grep Alpha")

	s := screen(m)
	for _, want := range []string{"alpha.md:1", "alpha.md:5"} {
		if !strings.Contains(s, want) {
			t.Errorf("screen lacks %q:\n%s", want, s)
		}
	}
	for _, unwanted := range []string{"index.md:3", "sub/gamma.md:3"} {
		if strings.Contains(s, unwanted) {
			t.Errorf("screen has %q, which only matches in lowercase:\n%s", unwanted, s)
		}
	}
}

func TestGrepSearchesTheUnsavedBuffer(t *testing.T) {
	m, _ := linked(t, "index.md")
	m = resize(m, 100, 30)
	m = press(typeKeys(m, "Ozebrafish"), esc)

	m = ex(m, "grep zebra")

	if s := screen(m); !strings.Contains(s, "index.md:1") || !strings.Contains(s, "zebrafish") {
		t.Errorf("the unsaved edit is not found:\n%s", s)
	}
}

func TestGrepUpdatesAsYouType(t *testing.T) {
	m, _ := linked(t, "index.md")
	m = resize(m, 100, 30)

	m = typeDriven(t, press(m, space), "/intro")

	s := screen(m)
	if !strings.Contains(s, "index.md:3") || !strings.Contains(s, "alpha.md:3") {
		t.Errorf("typed search shows no results:\n%s", s)
	}
	if strings.Contains(s, "beta.md") {
		t.Errorf("stale results for a shorter query:\n%s", s)
	}
}

func TestGrepEnterOpensAtTheMatchAndNFindsTheNext(t *testing.T) {
	m, v := linked(t, "index.md")
	m = resize(m, 100, 30)

	m = press(ex(m, "grep points"), enter)

	if got := v.rel(t, m); got != "alpha.md" {
		t.Fatalf("open Note = %q, want alpha.md", got)
	}
	if got := m.Cursor(); got != (engine.Pos{Line: 4, Col: 6}) {
		t.Errorf("cursor = %+v, want the match at 4:6", got)
	}
	m = ex(m, "grep alpha")
	m = press(m, enter) // alpha.md:1, "# Alpha"
	if got := m.Cursor(); got != (engine.Pos{Line: 0, Col: 2}) {
		t.Fatalf("cursor = %+v, want 0:2", got)
	}
	m = typeKeys(m, "n")
	if got := m.Cursor(); got != (engine.Pos{Line: 4, Col: 0}) {
		t.Errorf("after n, cursor = %+v, want the next match at 4:0", got)
	}
	m = press(press(m, ctrlO), ctrlO)
	if got := v.rel(t, m); got != "index.md" {
		t.Errorf("ctrl+o twice went to %q, want index.md", got)
	}
}

func TestGrepStopsAt200Results(t *testing.T) {
	m, v := linked(t, "index.md")
	if err := os.WriteFile(v.abs("many.md"), []byte(strings.Repeat("needle\n", 300)), 0o600); err != nil {
		t.Fatal(err)
	}
	v.ix.Update(v.abs("many.md"), []byte(strings.Repeat("needle\n", 300)))
	m = resize(m, 100, 30)

	m = ex(m, "grep needle")

	if s := screen(m); !strings.Contains(s, "200+ matches") {
		t.Errorf("no sign of the limit:\n%s", s)
	}
}

// msgsOf runs cmd and every command batched in it, returning their
// messages.
func msgsOf(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range batch {
			out = append(out, msgsOf(c)...)
		}
		return out
	}
	if msg == nil {
		return nil
	}
	return []tea.Msg{msg}
}

// update delivers each message in turn, collecting the commands returned.
func update(m app.Model, msgs []tea.Msg) (app.Model, tea.Cmd) {
	var cmds []tea.Cmd
	for _, msg := range msgs {
		next, cmd := m.Update(msg)
		m = next.(app.Model)
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

func TestGrepRunsOffTheUpdateLoopAndDropsStaleResults(t *testing.T) {
	m, _ := linked(t, "index.md")
	m = resize(typeKeys(m, " /bet"), 100, 30)
	next, tick := m.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = next.(app.Model)

	m, search := update(m, msgsOf(tick))

	if s := screen(m); strings.Contains(s, "index.md:4") {
		t.Fatalf("the search ran inside Update:\n%s", s)
	}
	if search == nil {
		t.Fatal("the debounce tick started no search")
	}
	results := msgsOf(search)
	m = press(m, tea.KeyPressMsg{Code: 'x', Text: "x"}) // the query is now "betax"
	m, _ = update(m, results)
	if s := screen(m); strings.Contains(s, "index.md:4") {
		t.Errorf("results for the old query were shown:\n%s", s)
	}

	m = press(m, tea.KeyPressMsg{Code: tea.KeyBackspace})
	next, tick = m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	m = drive(t, next.(app.Model), tick)
	if s := screen(m); !strings.Contains(s, "index.md:4") {
		t.Errorf("results for %q never arrived:\n%s", "bet", s)
	}
}
