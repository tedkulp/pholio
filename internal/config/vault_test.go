package config_test

import (
	"testing"

	"github.com/tedkulp/pholio/internal/config"
	"github.com/tedkulp/pholio/internal/seam/seamtest"
)

func TestFindVault(t *testing.T) {
	fsys := seamtest.NewMemFS(map[string]string{
		"/home/u/notes/a.md":                "",
		"/home/u/notes/sub/b.md":            "",
		"/home/u/other/.pholio/config.toml": "",
		"/home/u/other/deep/c.md":           "",
		"/home/u/obs/.obsidian/app.json":    "",
		"/home/u/obs/x/d.md":                "",
		"/home/u/loose/e.md":                "",
		"/home/u/plain.txt":                 "",
	})
	tests := []struct {
		name       string
		arg        string
		configured string
		want       config.Target
		wantErr    string
	}{
		{
			name: "directory argument is the Vault",
			arg:  "/home/u/notes",
			want: config.Target{Vault: "/home/u/notes"},
		},
		{
			name:       "directory argument beats the config",
			arg:        "/home/u/loose",
			configured: "/home/u/notes",
			want:       config.Target{Vault: "/home/u/loose"},
		},
		{
			name:       "no argument uses the configured Vault",
			configured: "/home/u/notes",
			want:       config.Target{Vault: "/home/u/notes"},
		},
		{
			name:       "configured Vault expands ~",
			configured: "~/notes",
			want:       config.Target{Vault: "/home/u/notes"},
		},
		{
			name:    "no argument and no config",
			wantErr: `no Vault: run "pholio <folder>" or set vault = "<folder>" in /home/u/.config/pholio/config.toml`,
		},
		{
			name:       "configured Vault missing",
			configured: "/home/u/gone",
			wantErr:    `Vault /home/u/gone (from /home/u/.config/pholio/config.toml) is not a folder`,
		},
		{
			name:       "file inside the configured Vault",
			arg:        "/home/u/notes/sub/b.md",
			configured: "/home/u/notes",
			want:       config.Target{Vault: "/home/u/notes", File: "/home/u/notes/sub/b.md"},
		},
		{
			name:       "file outside the configured Vault finds a .pholio folder above it",
			arg:        "/home/u/other/deep/c.md",
			configured: "/home/u/notes",
			want:       config.Target{Vault: "/home/u/other", File: "/home/u/other/deep/c.md"},
		},
		{
			name: "file finds a .obsidian folder above it",
			arg:  "/home/u/obs/x/d.md",
			want: config.Target{Vault: "/home/u/obs", File: "/home/u/obs/x/d.md"},
		},
		{
			name: "file with no marker uses its folder",
			arg:  "/home/u/loose/e.md",
			want: config.Target{Vault: "/home/u/loose", File: "/home/u/loose/e.md"},
		},
		{
			name:       "new .md file inside the configured Vault",
			arg:        "/home/u/notes/new.md",
			configured: "/home/u/notes",
			want:       config.Target{Vault: "/home/u/notes", File: "/home/u/notes/new.md"},
		},
		{
			name:    "missing path that is not a Note",
			arg:     "/home/u/nope",
			wantErr: `/home/u/nope: no such file or folder`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := config.FindVault(fsys, dirs, "/home/u", tt.arg, tt.configured)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}
