package config_test

import (
	"testing"

	"github.com/tedkulp/pholio/internal/config"
	"github.com/tedkulp/pholio/internal/seam/seamtest"
)

func TestStartupWithNoConfigFilesUsesDefaults(t *testing.T) {
	fsys := seamtest.NewMemFS(map[string]string{"/vault/a.md": "# A\n"})
	got, err := config.Startup(fsys, dirs, "/home/u", "/vault")
	if err != nil {
		t.Fatal(err)
	}
	if got.Target != (config.Target{Vault: "/vault"}) || got.Config != defaults() || got.Message != "" {
		t.Fatalf("got %+v", got)
	}
}

func TestStartupFindsVaultFromUserConfigThenReadsVaultConfig(t *testing.T) {
	fsys := seamtest.NewMemFS(map[string]string{
		userPath:                            "vault = \"~/notes\"\ndaily_folder = \"days\"\nbogus = 1\n",
		"/home/u/notes/.pholio/config.toml": "daily_folder = \"journal\"\n",
	})
	got, err := config.Startup(fsys, dirs, "/home/u", "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Target.Vault != "/home/u/notes" || got.Config.DailyFolder != "journal" {
		t.Fatalf("got %+v", got)
	}
	if want := "config: " + userPath + `: unknown key "bogus"`; got.Message != want {
		t.Fatalf("message = %q, want %q (reported once)", got.Message, want)
	}
}

func TestReloadRereadsBothFiles(t *testing.T) {
	fsys := seamtest.NewMemFS(map[string]string{userPath: "wrap = false\n", "/vault/a.md": ""})
	got, err := config.Startup(fsys, dirs, "/home/u", "/vault")
	if err != nil {
		t.Fatal(err)
	}
	if err := fsys.WriteFile(userPath, []byte("wrap = true\nmouse = false\n")); err != nil {
		t.Fatal(err)
	}
	if err := fsys.WriteFile(vaultPath, []byte("zettel_folder = \"z\"\n")); err != nil {
		t.Fatal(err)
	}
	cfg, msg := got.Reload(fsys)
	if !cfg.Wrap || cfg.Mouse || cfg.ZettelFolder != "z" || msg != "" {
		t.Fatalf("reload = %+v, %q", cfg, msg)
	}
}

func TestMessage(t *testing.T) {
	if got := config.Message(nil); got != "" {
		t.Errorf("no problems: %q", got)
	}
	if got, want := config.Message([]string{"a", "b"}), "config: 2 problems: a; b"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
