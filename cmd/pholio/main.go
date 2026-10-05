// Command pholio is a terminal markdown editor for a Vault of Notes.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"github.com/tedkulp/pholio/internal/app"
	"github.com/tedkulp/pholio/internal/config"
	"github.com/tedkulp/pholio/internal/seam"
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
	m, err := app.New(app.Deps{FS: fsys, Clock: clock}, path)
	if err != nil {
		return err
	}
	_, err = tea.NewProgram(m).Run()
	if s.Message != "" {
		// Until the status line can show it, report config problems on exit.
		fmt.Fprintln(os.Stderr, "pholio:", s.Message)
	}
	return err
}
