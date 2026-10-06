package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/tedkulp/pholio/internal/app"
	"github.com/tedkulp/pholio/internal/config"
)

// zettelOrigin starts on the daily fixture's README at monday 10:00, with
// the cursor at the end of its first line.
func zettelOrigin(t *testing.T) (string, app.Model) {
	t.Helper()
	vault, d := dailyVault(t)
	m := start(t, vault, d, filepath.Join(vault, "README.md"), monday)
	return vault, typeKeys(m, "$")
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

var enterKey = tea.KeyPressMsg{Code: tea.KeyEnter}

func TestSpcZPreviewsTheZettelFileName(t *testing.T) {
	_, m := zettelOrigin(t)

	m = keys(m, space)
	m = typeKeys(m, "z")
	if s := screen(m); !strings.Contains(s, "New Zettel") {
		t.Fatalf("no Zettel prompt:\n%s", s)
	}
	m = typeKeys(m, "Hello, World!")

	if s := screen(m); !strings.Contains(s, "zettel/20261005100000-hello-world.md") {
		t.Errorf("prompt does not preview the file name:\n%s", s)
	}
}

func TestZettelIsWrittenWithABackLinkAndLinkedFromTheOrigin(t *testing.T) {
	vault, m := zettelOrigin(t)

	m = keys(m, space)
	m = typeKeys(m, "z")
	m = typeKeys(m, "  Big   Idea #2 ")
	m = press(m, enterKey)

	zettel := filepath.Join(vault, "zettel", "20261005100000-big-idea-2.md")
	if got, want := readFile(t, zettel), "# Big   Idea #2\n\n[[README]]\n"; got != want {
		t.Errorf("Zettel on disk = %q, want %q", got, want)
	}
	wantOpen(t, m, "zettel/20261005100000-big-idea-2.md")
	if got, want := readFile(t, filepath.Join(vault, "README.md")),
		"# The daily Vault[[20261005100000-big-idea-2]]\n\nDaily Notes on 2026-09-28, 10-01, 10-02 and 10-08, with gaps between.\n"; got != want {
		t.Errorf("Origin on disk = %q, want %q", got, want)
	}
}

func TestZettelStampHasSeconds(t *testing.T) {
	vault, d := dailyVault(t)
	m := start(t, vault, d, filepath.Join(vault, "README.md"), time.Date(2026, 10, 6, 11, 55, 7, 0, time.Local))

	m = ex(m, "zettel My idea")

	wantOpen(t, m, "zettel/20261006115507-my-idea.md")
}

func TestZettelTimestampCollisionAddsASecond(t *testing.T) {
	vault, m := zettelOrigin(t)
	if err := os.MkdirAll(filepath.Join(vault, "zettel"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"20261005100000-other.md", "20261005100001.md", "20261005100003-x.md"} {
		if err := os.WriteFile(filepath.Join(vault, "zettel", name), []byte("# x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	m = ex(m, "zettel Next")

	wantOpen(t, m, "zettel/20261005100002-next.md")
	if !exists(filepath.Join(vault, "zettel", "20261005100002-next.md")) {
		t.Error("Zettel not written at the bumped second")
	}
}

func TestZettelIgnoresOldMinuteStamps(t *testing.T) {
	vault, m := zettelOrigin(t)
	old := filepath.Join(vault, "zettel", "202610051000-old.md")
	if err := os.MkdirAll(filepath.Dir(old), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(old, []byte("# old\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m = ex(m, "zettel New")

	wantOpen(t, m, "zettel/20261005100000-new.md")
	if got := readFile(t, old); got != "# old\n" {
		t.Errorf("old Zettel changed: %q", got)
	}
}

func TestZettelSlugRules(t *testing.T) {
	for title, want := range map[string]string{
		"Hello, World!":         "20261005100000-hello-world.md",
		"--Café  au   lait--":   "20261005100000-café-au-lait.md",
		"What's #1?":            "20261005100000-what-s-1.md",
		"!!!":                   "20261005100000.md",
		"Already-slugged_title": "20261005100000-already-slugged-title.md",
	} {
		_, m := zettelOrigin(t)
		m = keys(m, space)
		m = typeKeys(m, "z"+title)
		if s := screen(m); !strings.Contains(s, "zettel/"+want) {
			t.Errorf("%q: want preview %s in\n%s", title, want, s)
		}
	}
}

func TestZettelOpensWithAJumpBackToTheOrigin(t *testing.T) {
	_, m := zettelOrigin(t)

	m = ex(m, "zettel Idea")
	wantOpen(t, m, "zettel/20261005100000-idea.md")
	if got, want := m.Text(), "# Idea\n\n[[README]]\n"; got != want {
		t.Errorf("Zettel text = %q, want %q", got, want)
	}
	m = keys(m, tea.KeyPressMsg{Code: 'o', Mod: tea.ModCtrl})

	wantOpen(t, m, "README.md")
	if got := m.Cursor(); got.Line != 0 || got.Col != 39 {
		t.Errorf("cursor = %+v, want 0:39, the end of the inserted Link", got)
	}
}

func TestZettelFromADirtyOriginSavesItWithoutAsking(t *testing.T) {
	vault, m := zettelOrigin(t)
	m = typeKeys(m, "a!")
	m = keys(m, esc)

	m = ex(m, "zettel Idea")

	wantOpen(t, m, "zettel/20261005100000-idea.md")
	if got := readFile(t, filepath.Join(vault, "README.md")); !strings.HasPrefix(got, "# The daily Vault![[20261005100000-idea]]\n") {
		t.Errorf("Origin on disk = %q", got)
	}
}

func TestZettelNeedsANamedOrigin(t *testing.T) {
	vault, d := dailyVault(t)
	if err := os.MkdirAll(filepath.Join(d.ConfigHome, "pholio"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config.UserConfigPath(d), []byte("open_daily_on_startup = false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := start(t, vault, d, "", monday)

	m = ex(keys(m, ctrlL), "zettel Idea")

	if msg := messageLine(m); !strings.Contains(msg, ":w <name> first") {
		t.Errorf("message line %q", msg)
	}
	if got := m.Text(); got != "\n" {
		t.Errorf("text = %q, want the unnamed buffer untouched", got)
	}
}
