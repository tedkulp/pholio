// Command pholio is a terminal markdown editor for a Vault of Notes.
package main

import (
	"fmt"
	"os"
	pathpkg "path"
	"path/filepath"
	"runtime/debug"

	tea "charm.land/bubbletea/v2"

	"github.com/tedkulp/pholio/internal/app"
	"github.com/tedkulp/pholio/internal/config"
	"github.com/tedkulp/pholio/internal/index"
	"github.com/tedkulp/pholio/internal/seam"
	"github.com/tedkulp/pholio/internal/watch"
)

// version is stamped by the release build (.goreleaser.yaml) and `just build`.
// Left empty, versionString falls back to what the Go toolchain recorded.
var version string

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "pholio:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-v") {
		fmt.Println("pholio", versionString())
		return nil
	}
	if len(args) >= 1 && args[0] == "init" {
		return runInit(args[1:])
	}
	if len(args) > 1 {
		return errUsage
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

	// The index scans in the background once the program starts (app.Init).
	ix := index.New(fsys, s.Target.Vault, index.WithTemplatesDir(pathpkg.Dir(s.Config.DailyTemplate)))
	vault, watchErr := startWatching(fsys, s.Target.Vault, ix)
	defer func() { _ = vault.Close() }()
	// With no file argument this opens (or creates) today's Daily Note.
	deps := app.Deps{
		FS: fsys, Clock: clock, Watch: vault, Opener: seam.SystemOpener{}, Index: ix,
		Trash: seam.NewSystemTrash(os.Getenv, home, clock),
	}
	m, err := app.Start(deps, s)
	if err != nil {
		return err
	}
	if watchErr != nil {
		m = m.WithWatchError(watchErr)
	}
	_, err = tea.NewProgram(m).Run()
	return err
}

var errUsage = fmt.Errorf("usage: pholio [folder or file], or pholio init [folder]")

// runInit makes a folder (the current one if none is given) a Vault.
// To open a folder named init, run "pholio ./init".
func runInit(args []string) error {
	if len(args) > 1 {
		return errUsage
	}
	dir := "."
	if len(args) == 1 {
		dir = args[0]
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	lines, err := config.Init(seam.OSFS{}, config.DirsFromEnv(os.Getenv, home), home, abs)
	for _, l := range lines {
		fmt.Println(l)
	}
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

// versionString is the stamped version, else the module version Go recorded
// (the tag for `go install ...@v0.1.0`, a pseudo-version for a checkout).
func versionString() string {
	if version != "" {
		return version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" {
		return bi.Main.Version
	}
	return "(devel)"
}
