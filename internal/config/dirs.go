package config

import "path/filepath"

// Dirs are the base directories pholio's files live under.
type Dirs struct {
	// ConfigHome is $XDG_CONFIG_HOME, or ~/.config.
	ConfigHome string
	// StateHome is $XDG_STATE_HOME, or ~/.local/state.
	StateHome string
}

// DirsFromEnv resolves Dirs from the XDG variables, falling back to home.
// macOS uses the same fallbacks rather than ~/Library. Relative XDG values
// are ignored, as the XDG spec requires.
func DirsFromEnv(getenv func(string) string, home string) Dirs {
	pick := func(key string, fallback ...string) string {
		if v := getenv(key); filepath.IsAbs(v) {
			return v
		}
		return filepath.Join(append([]string{home}, fallback...)...)
	}
	return Dirs{
		ConfigHome: pick("XDG_CONFIG_HOME", ".config"),
		StateHome:  pick("XDG_STATE_HOME", ".local", "state"),
	}
}
