package config_test

import (
	"testing"
	"time"

	"github.com/tedkulp/pholio/internal/config"
	"github.com/tedkulp/pholio/internal/seam/seamtest"
)

var dirs = config.Dirs{ConfigHome: "/home/u/.config", StateHome: "/home/u/.local/state"}

const (
	userPath  = "/home/u/.config/pholio/config.toml"
	vaultPath = "/vault/.pholio/config.toml"
)

func defaults() config.Config {
	return config.Config{
		Theme:              "default",
		Mouse:              true,
		Wrap:               true,
		Conceal:            true,
		OpenDailyOnStartup: true,
		DayStartsAt:        0,
		DailyFolder:        "daily",
		DailyTemplate:      "templates/daily.md",
		ZettelFolder:       "zettel",
		NewNoteFolder:      "",
		TasksHeading:       "## Tasks",
	}
}

func TestLoadWithNoFilesGivesDefaults(t *testing.T) {
	fsys := seamtest.NewMemFS(nil)
	cfg, problems := config.Load(fsys, dirs, "/vault")
	if len(problems) != 0 {
		t.Fatalf("problems = %v, want none", problems)
	}
	if cfg != defaults() {
		t.Fatalf("cfg = %+v\nwant %+v", cfg, defaults())
	}
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name     string
		user     string
		vault    string
		want     func(*config.Config)
		problems []string
	}{
		{
			name: "user keys",
			user: "vault = \"/notes\"\ntheme = \"light\"\nmouse = false\nwrap = false\nconceal = false\nopen_daily_on_startup = false\n",
			want: func(c *config.Config) {
				c.Vault, c.Theme, c.Mouse, c.Wrap, c.Conceal, c.OpenDailyOnStartup = "/notes", "light", false, false, false, false
			},
		},
		{
			name: "vault keys in vault file",
			vault: `day_starts_at = "04:30"
daily_folder = "journal"
daily_template = "tpl/day.md"
zettel_folder = "z"
new_note_folder = "inbox"
tasks_heading = "### Todo"
`,
			want: func(c *config.Config) {
				c.DayStartsAt = 4*time.Hour + 30*time.Minute
				c.DailyFolder, c.DailyTemplate, c.ZettelFolder, c.NewNoteFolder, c.TasksHeading = "journal", "tpl/day.md", "z", "inbox", "### Todo"
			},
		},
		{
			name: "vault keys in user file apply",
			user: "daily_folder = \"days\"\nzettel_folder = \"zk\"\n",
			want: func(c *config.Config) { c.DailyFolder, c.ZettelFolder = "days", "zk" },
		},
		{
			name:  "vault file wins over user file",
			user:  "daily_folder = \"days\"\nzettel_folder = \"zk\"\n",
			vault: "daily_folder = \"journal\"\n",
			want:  func(c *config.Config) { c.DailyFolder, c.ZettelFolder = "journal", "zk" },
		},
		{
			name:     "unknown key in user file",
			user:     "colour = \"red\"\nwrap = false\n",
			want:     func(c *config.Config) { c.Wrap = false },
			problems: []string{userPath + `: unknown key "colour"`},
		},
		{
			name:     "user-only key in vault file is unknown",
			vault:    "theme = \"light\"\n",
			problems: []string{vaultPath + `: unknown key "theme"`},
		},
		{
			name:     "unknown table",
			user:     "[editor]\nwrap = false\n",
			problems: []string{userPath + `: unknown key "editor"`},
		},
		{
			name: "bad values fall back to defaults",
			user: "mouse = \"yes\"\ntheme = 3\nconceal = false\n",
			vault: `day_starts_at = "25:00"
daily_folder = "/abs"
zettel_folder = "../out"
tasks_heading = "Tasks"
`,
			want: func(c *config.Config) { c.Conceal = false },
			problems: []string{
				userPath + `: mouse: want true or false, got "yes"`,
				userPath + `: theme: want a string, got 3`,
				vaultPath + `: daily_folder: want a folder inside the Vault, got "/abs"`,
				vaultPath + `: day_starts_at: want "HH:MM", got "25:00"`,
				vaultPath + `: tasks_heading: want a markdown heading like "## Tasks", got "Tasks"`,
				vaultPath + `: zettel_folder: want a folder inside the Vault, got "../out"`,
			},
		},
		{
			name:     "bad TOML ignores that file only",
			user:     "wrap = \n",
			vault:    "daily_folder = \"journal\"\n",
			want:     func(c *config.Config) { c.DailyFolder = "journal" },
			problems: []string{userPath + ": toml: line 1 (last key \"wrap\"): expected value but found '\\n' instead"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := map[string]string{}
			if tt.user != "" {
				files[userPath] = tt.user
			}
			if tt.vault != "" {
				files[vaultPath] = tt.vault
			}
			cfg, problems := config.Load(seamtest.NewMemFS(files), dirs, "/vault")
			want := defaults()
			if tt.want != nil {
				tt.want(&want)
			}
			if cfg != want {
				t.Errorf("cfg = %+v\nwant  %+v", cfg, want)
			}
			if !equal(problems, tt.problems) {
				t.Errorf("problems:\n got %q\nwant %q", problems, tt.problems)
			}
		})
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
