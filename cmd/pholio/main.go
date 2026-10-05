// Command pholio is a terminal markdown editor for a Vault of Notes.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"github.com/tedkulp/pholio/internal/app"
	"github.com/tedkulp/pholio/internal/seam"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "pholio:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: pholio <file>")
	}
	path, err := filepath.Abs(args[0])
	if err != nil {
		return err
	}
	m, err := app.New(app.Deps{FS: seam.OSFS{}, Clock: seam.SystemClock{}}, path)
	if err != nil {
		return err
	}
	_, err = tea.NewProgram(m).Run()
	return err
}
