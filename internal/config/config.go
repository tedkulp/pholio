// Package config finds the Vault and loads pholio's two config files: the
// user config ($XDG_CONFIG_HOME/pholio/config.toml) and the Vault config
// (<vault>/.pholio/config.toml). It also reads and writes the app-written
// state file. Config files are only ever read.
//
// Every key has a default. Unknown keys and bad values are returned as
// problems, and those keys keep their defaults.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/tedkulp/pholio/internal/dates"
	"github.com/tedkulp/pholio/internal/seam"
)

// Config is the merged result of the user and Vault config files.
type Config struct {
	// User keys, read from the user config only.

	// Vault is the configured Vault path, used when no path is given on the
	// command line. Empty if unset.
	Vault              string
	Theme              string
	Mouse              bool
	Wrap               bool
	Conceal            bool
	OpenDailyOnStartup bool

	// Vault keys. They may be set in either file; the Vault file wins.

	// DayStartsAt is how long after midnight Today begins.
	DayStartsAt   time.Duration
	DailyFolder   string
	DailyTemplate string
	// DailySubfolder is a dates.Format layout, such as "YYYY/MM", for the
	// folders under DailyFolder that new Daily Notes go in. "" is none.
	DailySubfolder string
	ZettelFolder   string
	// NewNoteFolder is Vault-relative; "" is the Vault root.
	NewNoteFolder string
	TasksHeading  string
}

// Default returns the config used when no file sets anything.
func Default() Config {
	return Config{
		Theme:              "default",
		Mouse:              true,
		Wrap:               true,
		Conceal:            true,
		OpenDailyOnStartup: true,
		DailyFolder:        "daily",
		DailyTemplate:      "templates/daily.md",
		ZettelFolder:       "zettel",
		NewNoteFolder:      "",
		TasksHeading:       "## Tasks",
	}
}

// UserConfigPath is the path of the user config file.
func UserConfigPath(d Dirs) string {
	return filepath.Join(d.ConfigHome, "pholio", "config.toml")
}

// VaultConfigPath is the path of the Vault config file.
func VaultConfigPath(vault string) string {
	return filepath.Join(vault, ".pholio", "config.toml")
}

// Load reads the user config and, if vault is not empty, the Vault config,
// and merges them over the defaults. A missing file is not a problem. It is
// also the reload entry point (F8): call it again to pick up edits.
func Load(fsys seam.FS, d Dirs, vault string) (Config, []string) {
	cfg := Default()
	var problems []string
	problems = append(problems, apply(fsys, UserConfigPath(d), &cfg, userKeys, vaultKeys)...)
	if vault != "" {
		problems = append(problems, apply(fsys, VaultConfigPath(vault), &cfg, vaultKeys)...)
	}
	return cfg, problems
}

// LoadUser reads only the user config. Startup uses it to find the Vault
// before the Vault config can be read.
func LoadUser(fsys seam.FS, d Dirs) (Config, []string) {
	return Load(fsys, d, "")
}

// Message joins problems into the one-line startup message, or "" if there
// are none.
func Message(problems []string) string {
	switch len(problems) {
	case 0:
		return ""
	case 1:
		return "config: " + problems[0]
	}
	return fmt.Sprintf("config: %d problems: %s", len(problems), strings.Join(problems, "; "))
}

// setter checks a raw TOML value and stores it, or describes why it is bad.
type setter func(c *Config, v any) string

var userKeys = map[string]setter{
	"vault":                 str(func(c *Config) *string { return &c.Vault }, nil),
	"theme":                 str(func(c *Config) *string { return &c.Theme }, nonEmpty),
	"mouse":                 boolean(func(c *Config) *bool { return &c.Mouse }),
	"wrap":                  boolean(func(c *Config) *bool { return &c.Wrap }),
	"conceal":               boolean(func(c *Config) *bool { return &c.Conceal }),
	"open_daily_on_startup": boolean(func(c *Config) *bool { return &c.OpenDailyOnStartup }),
}

var vaultKeys = map[string]setter{
	"day_starts_at":   dayStart,
	"daily_folder":    str(func(c *Config) *string { return &c.DailyFolder }, inVault),
	"daily_template":  str(func(c *Config) *string { return &c.DailyTemplate }, inVaultFile),
	"daily_subfolder": str(func(c *Config) *string { return &c.DailySubfolder }, inVaultLayout),
	"zettel_folder":   str(func(c *Config) *string { return &c.ZettelFolder }, inVault),
	"new_note_folder": str(func(c *Config) *string { return &c.NewNoteFolder }, inVault),
	"tasks_heading":   str(func(c *Config) *string { return &c.TasksHeading }, heading),
}

func apply(fsys seam.FS, file string, cfg *Config, tables ...map[string]setter) []string {
	data, err := fsys.ReadFile(file)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return []string{fmt.Sprintf("%s: %v", file, err)}
	}
	var raw map[string]any
	if err := toml.Unmarshal(data, &raw); err != nil {
		return []string{fmt.Sprintf("%s: %v", file, err)}
	}
	keys := make([]string, 0, len(raw))
	for k := range raw {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var problems []string
	for _, k := range keys {
		set := lookup(k, tables)
		if set == nil {
			problems = append(problems, fmt.Sprintf("%s: unknown key %q", file, k))
			continue
		}
		if msg := set(cfg, raw[k]); msg != "" {
			problems = append(problems, fmt.Sprintf("%s: %s: %s", file, k, msg))
		}
	}
	return problems
}

func lookup(k string, tables []map[string]setter) setter {
	for _, t := range tables {
		if s, ok := t[k]; ok {
			return s
		}
	}
	return nil
}

func boolean(field func(*Config) *bool) setter {
	return func(c *Config, v any) string {
		b, ok := v.(bool)
		if !ok {
			return "want true or false, got " + show(v)
		}
		*field(c) = b
		return ""
	}
}

// str stores a string value, after check (if any) has accepted it. check
// returns the wanted shape when the value is bad.
func str(field func(*Config) *string, check func(string) string) setter {
	return func(c *Config, v any) string {
		s, ok := v.(string)
		if !ok {
			return "want a string, got " + show(v)
		}
		if check != nil {
			if want := check(s); want != "" {
				return fmt.Sprintf("want %s, got %q", want, s)
			}
		}
		*field(c) = s
		return ""
	}
}

func show(v any) string {
	if s, ok := v.(string); ok {
		return fmt.Sprintf("%q", s)
	}
	return fmt.Sprint(v)
}

func nonEmpty(s string) string {
	if s == "" {
		return "a non-empty string"
	}
	return ""
}

// inVault accepts a Vault-relative folder ("" is the Vault root).
func inVault(s string) string {
	if path.IsAbs(s) || filepath.IsAbs(s) {
		return "a folder inside the Vault"
	}
	if c := path.Clean(filepath.ToSlash(s)); c == ".." || strings.HasPrefix(c, "../") {
		return "a folder inside the Vault"
	}
	return ""
}

// layoutSample is any day; dates.Format tokens never render as a path
// separator or "..", so one day checks a layout for every day.
var layoutSample = time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)

// inVaultLayout checks a dates.Format folder layout, both as written and
// as rendered, since [bracketed] text is copied out as is.
func inVaultLayout(s string) string {
	if want := inVault(s); want != "" {
		return want
	}
	return inVault(dates.Format(layoutSample, s))
}

func inVaultFile(s string) string {
	if s == "" || inVault(s) != "" {
		return "a file inside the Vault"
	}
	return ""
}

var headingRE = regexp.MustCompile(`^#{1,6} \S`)

func heading(s string) string {
	if !headingRE.MatchString(s) {
		return `a markdown heading like "## Tasks"`
	}
	return ""
}

var clockRE = regexp.MustCompile(`^([01]\d|2[0-3]):([0-5]\d)$`)

func dayStart(c *Config, v any) string {
	s, ok := v.(string)
	if !ok {
		return "want \"HH:MM\", got " + show(v)
	}
	m := clockRE.FindStringSubmatch(s)
	if m == nil {
		return fmt.Sprintf("want \"HH:MM\", got %q", s)
	}
	var h, mm int
	_, _ = fmt.Sscanf(m[1]+" "+m[2], "%d %d", &h, &mm)
	c.DayStartsAt = time.Duration(h)*time.Hour + time.Duration(mm)*time.Minute
	return ""
}
