package config_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/tedkulp/pholio/internal/config"
	"github.com/tedkulp/pholio/internal/seam/seamtest"
)

const vaultCfg = "/home/u/v/.pholio/config.toml"

func read(t *testing.T, fsys *seamtest.MemFS, p string) string {
	t.Helper()
	b, err := fsys.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestInitCreatesAVault(t *testing.T) {
	fsys := seamtest.NewMemFS(nil)
	lines, err := config.Init(fsys, dirs, "/home/u", "/home/u/v")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"created ~/v",
		"created ~/v/daily",
		"created ~/v/zettel",
		"created ~/v/templates/daily.md",
		"created ~/v/.pholio/config.toml",
		`added vault = "~/v" to ~/.config/pholio/config.toml`,
	}
	if !reflect.DeepEqual(lines, want) {
		t.Fatalf("lines:\n%s\nwant:\n%s", strings.Join(lines, "\n"), strings.Join(want, "\n"))
	}
	for _, d := range []string{"/home/u/v/daily", "/home/u/v/zettel"} {
		if info, err := fsys.Stat(d); err != nil || !info.IsDir() {
			t.Errorf("%s is not a folder: %v", d, err)
		}
	}
	if got := read(t, fsys, "/home/u/v/templates/daily.md"); got != "# {{title}}\n\n## Tasks\n" {
		t.Errorf("template = %q", got)
	}
	if got := read(t, fsys, userPath); got != "vault = \"~/v\"\n" {
		t.Errorf("user config = %q", got)
	}
	s, err := config.Startup(fsys, dirs, "/home/u", "")
	if err != nil {
		t.Fatal(err)
	}
	if s.Target.Vault != "/home/u/v" || s.Message != "" {
		t.Fatalf("startup after init: %+v", s)
	}
}

func TestInitConfigListsEveryVaultKeyAtItsDefault(t *testing.T) {
	text := config.InitVaultConfig()
	for _, k := range config.VaultKeyNames() {
		if !strings.Contains(text, "\n# "+k+" = ") {
			t.Errorf("vault key %q is missing from the init config", k)
		}
	}
	// As written, and with any one key uncommented, it loads as the defaults.
	variants := []string{text}
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "# ") && strings.Contains(line, " = ") {
			variants = append(variants, strings.Replace(text, line, strings.TrimPrefix(line, "# "), 1))
		}
	}
	if len(variants) != len(config.VaultKeyNames())+1 {
		t.Fatalf("found %d key lines, want %d", len(variants)-1, len(config.VaultKeyNames()))
	}
	for _, v := range variants {
		fsys := seamtest.NewMemFS(map[string]string{"/vault/.pholio/config.toml": v})
		cfg, problems := config.Load(fsys, dirs, "/vault")
		if len(problems) != 0 || cfg != defaults() {
			t.Errorf("loading:\n%s\ngot %+v, problems %v", v, cfg, problems)
		}
	}
}

func TestInitTwiceChangesNothing(t *testing.T) {
	fsys := seamtest.NewMemFS(nil)
	if _, err := config.Init(fsys, dirs, "/home/u", "/home/u/v"); err != nil {
		t.Fatal(err)
	}
	before := map[string]string{}
	for _, p := range []string{userPath, vaultCfg, "/home/u/v/templates/daily.md"} {
		before[p] = read(t, fsys, p)
	}
	lines, err := config.Init(fsys, dirs, "/home/u", "/home/u/v")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"~/v is already a Vault"}; !reflect.DeepEqual(lines, want) {
		t.Fatalf("lines = %q, want %q", lines, want)
	}
	for p, s := range before {
		if got := read(t, fsys, p); got != s {
			t.Errorf("%s changed to %q", p, got)
		}
	}
}

func TestInitKeepsExistingFilesAndFillsGaps(t *testing.T) {
	fsys := seamtest.NewMemFS(map[string]string{
		vaultCfg:                       "daily_folder = \"days\"\n",
		"/home/u/v/templates/daily.md": "mine\n",
		userPath:                       "# my config\ntheme = \"dark\"",
	})
	lines, err := config.Init(fsys, dirs, "/home/u", "/home/u/v")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"~/v is already a Vault",
		"created ~/v/daily",
		"created ~/v/zettel",
		`added vault = "~/v" to ~/.config/pholio/config.toml`,
	}
	if !reflect.DeepEqual(lines, want) {
		t.Fatalf("lines = %q, want %q", lines, want)
	}
	if got := read(t, fsys, vaultCfg); got != "daily_folder = \"days\"\n" {
		t.Errorf("vault config = %q", got)
	}
	if got := read(t, fsys, "/home/u/v/templates/daily.md"); got != "mine\n" {
		t.Errorf("template = %q", got)
	}
	if got := read(t, fsys, userPath); got != "# my config\ntheme = \"dark\"\nvault = \"~/v\"\n" {
		t.Errorf("user config = %q", got)
	}
}

func TestInitWithVaultAlreadySetLeavesUserConfig(t *testing.T) {
	fsys := seamtest.NewMemFS(map[string]string{userPath: "vault = \"~/notes\"\n"})
	lines, err := config.Init(fsys, dirs, "/home/u", "/srv/v")
	if err != nil {
		t.Fatal(err)
	}
	last := lines[len(lines)-1]
	if want := `vault is already set to "~/notes"; set vault = "/srv/v" in ~/.config/pholio/config.toml to use this one`; last != want {
		t.Errorf("last line = %q, want %q", last, want)
	}
	if got := read(t, fsys, userPath); got != "vault = \"~/notes\"\n" {
		t.Errorf("user config = %q", got)
	}
}

func TestInitRefuses(t *testing.T) {
	tests := []struct {
		name    string
		files   map[string]string
		dir     string
		wantErr string
		isFile  bool
	}{
		{
			name:    "a file",
			files:   map[string]string{"/home/u/notes.md": ""},
			dir:     "/home/u/notes.md",
			wantErr: "~/notes.md is a file, not a folder",
			isFile:  true,
		},
		{
			name:    "inside the configured Vault",
			files:   map[string]string{userPath: "vault = \"~/notes\"\n"},
			dir:     "/home/u/notes/projects",
			wantErr: "~/notes is already a Vault; projects is inside it",
		},
		{
			name:    "below a .pholio folder",
			files:   map[string]string{"/srv/v/.pholio/config.toml": ""},
			dir:     "/srv/v/a/b",
			wantErr: "/srv/v is already a Vault; a/b is inside it",
		},
		{
			name:    "below a .obsidian folder",
			files:   map[string]string{"/srv/o/.obsidian/app.json": ""},
			dir:     "/srv/o/x",
			wantErr: "/srv/o is already a Vault; x is inside it",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsys := seamtest.NewMemFS(tt.files)
			_, err := config.Init(fsys, dirs, "/home/u", tt.dir)
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("err = %v, want %q", err, tt.wantErr)
			}
			if _, err := fsys.Stat(tt.dir); !tt.isFile && err == nil {
				t.Errorf("%s was created", tt.dir)
			}
			if _, ok := tt.files[userPath]; !ok {
				if _, err := fsys.Stat(userPath); err == nil {
					t.Errorf("user config was created")
				}
			}
		})
	}
}

func TestInitOnTheConfiguredVaultRootIsAllowed(t *testing.T) {
	fsys := seamtest.NewMemFS(map[string]string{userPath: "vault = \"~/notes\"\n", "/home/u/notes/.obsidian/app.json": ""})
	lines, err := config.Init(fsys, dirs, "/home/u", "/home/u/notes")
	if err != nil {
		t.Fatal(err)
	}
	if lines[len(lines)-1] != "created ~/notes/.pholio/config.toml" {
		t.Errorf("lines = %q", lines)
	}
}

func TestInitLeavesAUserConfigItCannotSafelyAppendTo(t *testing.T) {
	tests := []struct{ name, user, wantLast string }{
		{"empty vault", "vault = \"\"\n", `vault is already set to ""; set vault = "/srv/v" in ~/.config/pholio/config.toml to use this one`},
		{"bad TOML", "vault = \n", `~/.config/pholio/config.toml does not parse; set vault = "/srv/v" in it to use this one`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsys := seamtest.NewMemFS(map[string]string{userPath: tt.user})
			lines, err := config.Init(fsys, dirs, "/home/u", "/srv/v")
			if err != nil {
				t.Fatal(err)
			}
			if last := lines[len(lines)-1]; last != tt.wantLast {
				t.Errorf("last line = %q, want %q", last, tt.wantLast)
			}
			if got := read(t, fsys, userPath); got != tt.user {
				t.Errorf("user config = %q", got)
			}
		})
	}
}
