package config_test

import (
	"testing"

	"github.com/tedkulp/pholio/internal/config"
	"github.com/tedkulp/pholio/internal/seam/seamtest"
)

const statePath = "/home/u/.local/state/pholio/state.toml"

func TestStateDefaultsWhenMissing(t *testing.T) {
	s := config.NewStateStore(seamtest.NewMemFS(nil), dirs)
	got, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got != (config.State{SidebarWidth: 30}) {
		t.Fatalf("got %+v", got)
	}
}

func TestStateRoundTrips(t *testing.T) {
	fsys := seamtest.NewMemFS(nil)
	s := config.NewStateStore(fsys, dirs)
	if err := s.Save(config.State{SidebarWidth: 42}); err != nil {
		t.Fatal(err)
	}
	got, err := config.NewStateStore(fsys, dirs).Load()
	if err != nil {
		t.Fatal(err)
	}
	if got != (config.State{SidebarWidth: 42}) {
		t.Fatalf("got %+v", got)
	}
	if _, err := fsys.Stat(statePath); err != nil {
		t.Fatalf("state not at %s: %v", statePath, err)
	}
}

func TestStateBadFileGivesDefaultsAndAnError(t *testing.T) {
	for name, body := range map[string]string{
		"bad toml":  "sidebar_width = \n",
		"bad value": "sidebar_width = \"wide\"\n",
		"negative":  "sidebar_width = -4\n",
	} {
		t.Run(name, func(t *testing.T) {
			fsys := seamtest.NewMemFS(map[string]string{statePath: body})
			got, err := config.NewStateStore(fsys, dirs).Load()
			if err == nil {
				t.Error("want an error")
			}
			if got != (config.State{SidebarWidth: 30}) {
				t.Errorf("got %+v, want defaults", got)
			}
		})
	}
}

func TestConfigFilesAreNeverWritten(t *testing.T) {
	user := "wrap = false\nbogus = 1\n"
	vault := "daily_folder = \"journal\"\n"
	fsys := seamtest.NewMemFS(map[string]string{userPath: user, vaultPath: vault})

	_, _ = config.Load(fsys, dirs, "/vault")
	if err := config.NewStateStore(fsys, dirs).Save(config.State{SidebarWidth: 50}); err != nil {
		t.Fatal(err)
	}
	_, _ = config.Load(fsys, dirs, "/vault")

	for p, want := range map[string]string{userPath: user, vaultPath: vault} {
		got, err := fsys.ReadFile(p)
		if err != nil || string(got) != want {
			t.Errorf("%s = %q, %v; want %q unchanged", p, got, err, want)
		}
	}
}
