package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/tedkulp/pholio/internal/index"
	"github.com/tedkulp/pholio/internal/seam"
)

// Target is what pholio was asked to open: a Vault, and optionally one file
// in it.
type Target struct {
	// Vault is the absolute path of the Vault folder.
	Vault string
	// File is the absolute path of the file to open, or "" to open the
	// default (today's Daily Note).
	File string
}

// vaultMarkers are folders that mark a Vault root when pholio is given a
// file outside the configured Vault.
var vaultMarkers = []string{".pholio", ".obsidian"}

// FindVault works out the Vault from the command-line path arg (absolute, or
// "" if none) and the user config's vault value (configured, may start with
// ~). A folder argument is the Vault. A file argument (which may not exist
// yet if it ends in .md) belongs to the configured Vault if it is inside it,
// else to the nearest folder above it holding .pholio or .obsidian, else to
// its own folder.
func FindVault(fsys seam.FS, d Dirs, home, arg, configured string) (Target, error) {
	configured = expandHome(configured, home)
	if arg == "" {
		if configured == "" {
			return Target{}, fmt.Errorf(`no Vault: run "pholio <folder>" or set vault = "<folder>" in %s`, UserConfigPath(d))
		}
		if !isDir(fsys, configured) {
			return Target{}, fmt.Errorf("Vault %s (from %s) is not a folder", configured, UserConfigPath(d)) //nolint:revive,staticcheck // Vault is a proper noun
		}
		return Target{Vault: configured}, nil
	}
	arg = filepath.Clean(arg)
	info, err := fsys.Stat(arg)
	switch {
	case err == nil && info.IsDir():
		return Target{Vault: arg}, nil
	case err != nil && !index.IsNote(arg):
		return Target{}, fmt.Errorf("%s: no such file or folder", arg)
	}
	t := Target{File: arg}
	if configured != "" && within(configured, arg) {
		t.Vault = configured
		return t, nil
	}
	t.Vault = filepath.Dir(arg)
	for dir := t.Vault; ; dir = filepath.Dir(dir) {
		for _, m := range vaultMarkers {
			if isDir(fsys, filepath.Join(dir, m)) {
				t.Vault = dir
				return t, nil
			}
		}
		if dir == filepath.Dir(dir) {
			return t, nil
		}
	}
}

func isDir(fsys seam.FS, p string) bool {
	info, err := fsys.Stat(p)
	return err == nil && info.IsDir()
}

func within(dir, p string) bool {
	rel, err := filepath.Rel(filepath.Clean(dir), p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func expandHome(p, home string) string {
	switch {
	case p == "~":
		return home
	case strings.HasPrefix(p, "~/"):
		return filepath.Join(home, p[2:])
	case p == "":
		return ""
	}
	return filepath.Clean(p)
}
