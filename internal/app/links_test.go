package app_test

import (
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/tedkulp/pholio/internal/app"
	"github.com/tedkulp/pholio/internal/config"
	"github.com/tedkulp/pholio/internal/index"
	"github.com/tedkulp/pholio/internal/seam"
	"github.com/tedkulp/pholio/internal/seam/seamtest"
	"github.com/tedkulp/pholio/internal/testutil"
)

// linksVault is a copy of the links fixture with the app open on
// index.md, its index scanned and a fake Opener.
type linksVault struct {
	root   string
	opener *seamtest.Opener
	ix     *index.Index
}

func (v linksVault) abs(rel string) string { return filepath.Join(v.root, filepath.FromSlash(rel)) }

// linked opens file (Vault-relative) in a fresh copy of the links Vault and
// runs the startup index scan.
func linked(t *testing.T, file string) (app.Model, linksVault) {
	t.Helper()
	v := linksVault{root: testutil.CopyVault(t, "links"), opener: &seamtest.Opener{}}
	m := openIn(t, &v, file)
	m = drive(t, m, m.Init())
	return m, v
}

// openIn starts the app on file in v without running any command.
func openIn(t *testing.T, v *linksVault, file string) app.Model {
	t.Helper()
	fsys := seam.OSFS{}
	v.ix = index.New(fsys, v.root)
	s, err := config.Startup(fsys, dirs, "/home/u", v.abs(file))
	if err != nil {
		t.Fatal(err)
	}
	m, err := app.New(app.Deps{FS: fsys, Opener: v.opener, Index: v.ix}, v.abs(file))
	if err != nil {
		t.Fatal(err)
	}
	return resize(m.WithSession(s), 80, 12)
}

// drive runs cmd the way the program would, feeding every message it
// produces (batches included) back into the model.
func drive(t *testing.T, m app.Model, cmd tea.Cmd) app.Model {
	t.Helper()
	if cmd == nil {
		return m
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			m = drive(t, m, c)
		}
		return m
	}
	if msg == nil {
		return m
	}
	next, more := m.Update(msg)
	return drive(t, next.(app.Model), more)
}

// search moves the cursor to the next match of s.
func search(m app.Model, s string) app.Model {
	return press(typeKeys(m, "/"+s), enter)
}

func (v linksVault) rel(t *testing.T, m app.Model) string {
	t.Helper()
	r, err := filepath.Rel(v.root, m.Path())
	if err != nil {
		t.Fatal(err)
	}
	return filepath.ToSlash(r)
}

func TestEnterFollowsAWikiLink(t *testing.T) {
	m, v := linked(t, "index.md")
	m = press(search(m, "alpha]]"), enter)
	if got := v.rel(t, m); got != "alpha.md" {
		t.Fatalf("open Note = %q, want alpha.md", got)
	}
}
