package config_test

import (
	"testing"

	"github.com/tedkulp/pholio/internal/config"
)

func TestDirsFromEnv(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want config.Dirs
	}{
		{
			name: "unset falls back to home, on macOS too",
			want: config.Dirs{ConfigHome: "/home/u/.config", StateHome: "/home/u/.local/state"},
		},
		{
			name: "XDG vars win",
			env:  map[string]string{"XDG_CONFIG_HOME": "/xdg/config", "XDG_STATE_HOME": "/xdg/state"},
			want: config.Dirs{ConfigHome: "/xdg/config", StateHome: "/xdg/state"},
		},
		{
			name: "relative XDG vars are ignored, per the XDG spec",
			env:  map[string]string{"XDG_CONFIG_HOME": "rel/config", "XDG_STATE_HOME": "rel"},
			want: config.Dirs{ConfigHome: "/home/u/.config", StateHome: "/home/u/.local/state"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := config.DirsFromEnv(func(k string) string { return tt.env[k] }, "/home/u")
			if got != tt.want {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}
