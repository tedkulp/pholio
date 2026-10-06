package app_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"

	"github.com/tedkulp/pholio/internal/app"
)

// leaderKeys are every Leader key with its help text.
var leaderKeys = []string{
	"e → sidebar", "d → today", "D → date", "b → backlinks", "/ → search",
	"z → zettel", "f → find", "n → new", "t → tasks",
}

func deliver(m app.Model, msg tea.Msg) app.Model {
	next, _ := m.Update(msg)
	return next.(app.Model)
}

// waitLeader presses spc and lets the which-key delay pass.
func waitLeader(m app.Model) app.Model {
	m = keys(m, space)
	return deliver(m, app.LeaderTick(m))
}

func TestSpcStartsTheWhichKeyDelay(t *testing.T) {
	m := shell(t, shellVault(), "/vault/a.md")

	_, cmd := m.Update(space)

	if cmd == nil {
		t.Error("spc returned no command: nothing will open the popup")
	}
	if strings.Contains(screen(keys(m, space)), "d → today") {
		t.Error("popup drawn before the delay")
	}
}

func TestWhichKeyPopupAboveTheStatusLine(t *testing.T) {
	m := waitLeader(shell(t, shellVault(), "/vault/a.md"))

	golden.RequireEqual(t, screen(m))
	s := screen(m)
	for _, want := range leaderKeys {
		if !strings.Contains(s, want) {
			t.Errorf("popup lacks %q:\n%s", want, s)
		}
	}
	if !strings.Contains(rows(m)[10], "e sidebar") {
		t.Errorf("status line = %q, want the leader hint kept", rows(m)[10])
	}
}

func TestLeaderKeyBeforeTheDelayNeverDrawsThePopup(t *testing.T) {
	m := keys(shell(t, shellVault(), "/vault/a.md"), space)
	tick := app.LeaderTick(m)

	m = deliver(typeKeys(m, "d"), tick)

	if strings.Contains(screen(m), "→") {
		t.Errorf("popup drawn after the leader key ran:\n%s", screen(m))
	}
	if m.Path() == "/vault/a.md" {
		t.Error("spc d did not open today's Daily Note")
	}
}

func TestStaleTickDoesNotOpenThePopupForALaterSpc(t *testing.T) {
	m := keys(shell(t, shellVault(), "/vault/a.md"), space)
	stale := app.LeaderTick(m)

	m = deliver(keys(m, esc, space), stale)

	if strings.Contains(screen(m), "→") {
		t.Errorf("stale tick opened the popup:\n%s", screen(m))
	}
}

func TestLeaderKeyRunsAndClosesThePopup(t *testing.T) {
	m := waitLeader(shell(t, shellVault(), "/vault/a.md"))

	m = typeKeys(m, "e")

	if strings.Contains(screen(m), "→") {
		t.Errorf("popup still open:\n%s", screen(m))
	}
	if strings.Contains(screen(m), "▸ daily") {
		t.Error("spc e did not hide the sidebar")
	}
}

func TestEscClosesThePopup(t *testing.T) {
	m := waitLeader(shell(t, shellVault(), "/vault/a.md"))

	m = keys(m, esc)

	if strings.Contains(screen(m), "→") {
		t.Errorf("popup still open:\n%s", screen(m))
	}
	if m.Path() != "/vault/a.md" || m.Text() != "# Alpha\nfirst note\n" {
		t.Error("esc ran something")
	}
}

func TestWhichKeyWrapsAtNarrowWidths(t *testing.T) {
	m := waitLeader(resize(shell(t, shellVault(), "/vault/a.md"), 40, 12))

	s := screen(m)
	for _, want := range leaderKeys {
		if !strings.Contains(s, want) {
			t.Errorf("popup lacks %q at 40 columns:\n%s", want, s)
		}
	}
	for i, row := range rows(m) {
		if w := ansi.StringWidth(row); w > 40 {
			t.Errorf("row %d is %d cells", i, w)
		}
	}
}

func TestWhichKeyFitsAShortScreen(t *testing.T) {
	m := waitLeader(resize(shell(t, shellVault(), "/vault/a.md"), 20, 5))

	r := rows(m)
	if len(r) != 5 {
		t.Fatalf("%d rows, want 5:\n%s", len(r), screen(m))
	}
	if !strings.Contains(screen(m), "→") {
		t.Errorf("no popup rows drawn:\n%s", screen(m))
	}
}
