// Command pholio is a terminal markdown editor for a Vault of Notes.
package main

import (
	"fmt"
	"os"
	pathpkg "path"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"github.com/tedkulp/pholio/internal/app"
	"github.com/tedkulp/pholio/internal/config"
	"github.com/tedkulp/pholio/internal/index"
	"github.com/tedkulp/pholio/internal/seam"
	"github.com/tedkulp/pholio/internal/watch"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "pholio:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("usage: pholio [folder or file]")
	}
	var arg string
	if len(args) == 1 {
		abs, err := filepath.Abs(args[0])
		if err != nil {
			return err
		}
		arg = abs
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	fsys, clock := seam.OSFS{}, seam.SystemClock{}
	s, err := config.Startup(fsys, config.DirsFromEnv(os.Getenv, home), home, arg)
	if err != nil {
		return err
	}

	path := s.Target.File
	if path == "" {
		// Placeholder until the Daily Notes ticket: open today's Daily Note
		// path (Today shifted by day_starts_at) without a template.
		today := clock.Now().Add(-s.Config.DayStartsAt).Format("2006-01-02")
		path = filepath.Join(s.Target.Vault, s.Config.DailyFolder, today+".md")
	}
	// The index scans in the background once the program starts (app.Init).
	ix := index.New(fsys, s.Target.Vault, index.WithTemplatesDir(pathpkg.Dir(s.Config.DailyTemplate)))
	vault, watchErr := startWatching(fsys, s.Target.Vault, ix)
	defer func() { _ = vault.Close() }()
	deps := app.Deps{FS: fsys, Clock: clock, Watch: vault, Opener: seam.SystemOpener{}, Index: ix}
	m, err := app.New(deps, path)
	if err != nil {
		return err
	}
	m = m.WithSession(s)
	if watchErr != nil {
		m = m.WithWatchError(watchErr)
	}
	_, err = tea.NewProgram(m).Run()
	return err
}

// startWatching watches the Vault for changes made outside pholio. If that
// fails (macOS can run out of descriptors even after raising the limit),
// the Vault still rescans on terminal focus and the error is returned for
// a warning.
func startWatching(fsys seam.FS, root string, ix *index.Index) (*watch.Vault, error) {
	_ = watch.RaiseFileLimit()
	v := watch.New(fsys, root, watch.Options{Index: ix})
	w, err := watch.NewFSNotify()
	if err == nil {
		err = v.Watch(w)
	}
	return v, err
}
