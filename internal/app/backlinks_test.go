package app_test

import (
	"strings"
	"testing"

	"github.com/tedkulp/pholio/internal/engine"
)

func TestBacklinksListTheNotesLinkingHere(t *testing.T) {
	m, _ := linked(t, "alpha.md")
	m = resize(m, 100, 24)

	m = typeKeys(press(m, space), "b")

	s := screen(m)
	for _, want := range []string{"Backlinks", "index.md:3", "Start at [[alpha]]", "sub/gamma.md:3", "Back up to [alpha](../alpha.md)."} {
		if !strings.Contains(s, want) {
			t.Errorf("screen lacks %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, "sync-conflict") || strings.Contains(s, "Conflict copy") {
		t.Errorf("a Conflict Note is listed:\n%s", s)
	}
}

func TestBacklinksFilterAndEnterJumpsToTheLink(t *testing.T) {
	m, v := linked(t, "alpha.md")
	m = resize(m, 100, 24)

	m = press(typeKeys(press(m, space), "bgamma"), enter)

	if got := v.rel(t, m); got != "sub/gamma.md" {
		t.Fatalf("open Note = %q, want sub/gamma.md", got)
	}
	if got := m.Cursor(); got != (engine.Pos{Line: 2, Col: 11}) {
		t.Errorf("cursor = %+v, want the Link at 2:11", got)
	}
	m = press(m, ctrlO)
	if got := v.rel(t, m); got != "alpha.md" {
		t.Errorf("ctrl+o went to %q, want back to alpha.md", got)
	}
}

func TestBacklinksExCommand(t *testing.T) {
	m, _ := linked(t, "deep/er/beta.md")
	m = resize(m, 100, 24)

	m = ex(m, "backlinks")

	s := screen(m)
	if !strings.Contains(s, "index.md:4") || !strings.Contains(s, "alpha.md:5") {
		t.Errorf("screen lacks beta's Backlinks:\n%s", s)
	}
}
