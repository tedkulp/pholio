package seam

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestPickClipboard(t *testing.T) {
	for _, c := range []struct {
		name, goos string
		env        map[string]string
		path       []string // tools on $PATH
		want       string   // the copy tool, or "" for none
	}{
		{"macOS", "darwin", nil, []string{"pbcopy"}, "pbcopy"},
		{"Wayland", "linux", map[string]string{"WAYLAND_DISPLAY": "wayland-0", "DISPLAY": ":0"}, []string{"wl-copy", "xclip"}, "wl-copy"},
		{"Wayland without wl-copy uses XWayland", "linux", map[string]string{"WAYLAND_DISPLAY": "wayland-0", "DISPLAY": ":0"}, []string{"xclip"}, "xclip"},
		{"X11 xclip", "linux", map[string]string{"DISPLAY": ":0"}, []string{"xclip", "xsel"}, "xclip"},
		{"X11 xsel", "linux", map[string]string{"DISPLAY": ":0"}, []string{"xsel"}, "xsel"},
		{"no display (SSH)", "linux", nil, []string{"wl-copy", "xclip"}, ""},
		{"display but no tool", "linux", map[string]string{"DISPLAY": ":0"}, nil, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			getenv := func(k string) string { return c.env[k] }
			lookPath := func(name string) (string, error) {
				if slices.Contains(c.path, name) {
					return "/usr/bin/" + name, nil
				}
				return "", errors.New("not found")
			}
			cb := pickClipboard(c.goos, getenv, lookPath)
			got := ""
			if cb.copy != nil {
				got = cb.copy[0]
			}
			if got != c.want {
				t.Errorf("copy tool = %q, want %q", got, c.want)
			}
			if (cb.copy == nil) != (cb.paste == nil) {
				t.Errorf("copy %q but paste %q", cb.copy, cb.paste)
			}
		})
	}
}

func TestNoClipboardTool(t *testing.T) {
	var cb SystemClipboard
	if err := cb.Write("x"); !errors.Is(err, ErrNoClipboard) {
		t.Errorf("Write: %v", err)
	}
	if _, err := cb.Read(); !errors.Is(err, ErrNoClipboard) {
		t.Errorf("Read: %v", err)
	}
}

func TestSystemClipboardRunsTheTools(t *testing.T) {
	f := filepath.Join(t.TempDir(), "clip")
	cb := SystemClipboard{copy: []string{"sh", "-c", `cat > "$0"`, f}, paste: []string{"cat", f}}
	if err := cb.Write("a\nb"); err != nil {
		t.Fatal(err)
	}
	got, err := cb.Read()
	if err != nil || got != "a\nb" {
		t.Errorf("Read = %q, %v", got, err)
	}
}

// shortTempDir is a temporary directory with a path short enough for unix
// sockets: macOS allows 104 bytes, and t.TempDir there is too long.
func shortTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "wl")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

// socket makes a unix socket at dir/name. A live one accepts connections
// until the test ends; a stale one is left behind with nothing listening.
func socket(t *testing.T, dir, name string, live bool) {
	t.Helper()
	l, err := net.Listen("unix", filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	if live {
		t.Cleanup(func() { _ = l.Close() })
		return
	}
	l.(*net.UnixListener).SetUnlinkOnClose(false)
	_ = l.Close()
}

func TestFallbackDisplay(t *testing.T) {
	for _, c := range []struct {
		name          string
		goos          string // "" is linux
		live, stale   []string
		files         []string
		display, want string
	}{
		{name: "one live socket", live: []string{"wayland-1"}, files: []string{"wayland-1.lock"}, want: "wayland-1"},
		{name: "stale socket skipped", live: []string{"wayland-1"}, stale: []string{"wayland-0"}, want: "wayland-1"},
		{name: "two live sockets", live: []string{"wayland-0", "wayland-1"}, want: ""},
		{name: "only stale", stale: []string{"wayland-0"}, want: ""},
		{name: "lock file is not a socket", files: []string{"wayland-1"}, want: ""},
		{name: "other sockets ignored", live: []string{"wayland-1", "herdr-nvim"}, want: "wayland-1"},
		{name: "a display is already set", live: []string{"wayland-1"}, display: ":0", want: ""},
		{name: "not on macOS", goos: "darwin", live: []string{"wayland-1"}, want: ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := shortTempDir(t)
			for _, n := range c.live {
				socket(t, dir, n, true)
			}
			for _, n := range c.stale {
				socket(t, dir, n, false)
			}
			for _, n := range c.files {
				if err := os.WriteFile(filepath.Join(dir, n), nil, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			env := map[string]string{"XDG_RUNTIME_DIR": dir, "DISPLAY": c.display}
			goos := c.goos
			if goos == "" {
				goos = "linux"
			}
			if got := fallbackDisplay(goos, func(k string) string { return env[k] }); got != c.want {
				t.Errorf("fallbackDisplay = %q, want %q", got, c.want)
			}
		})
	}
}

func TestToolsGetTheFallbackDisplay(t *testing.T) {
	cb := SystemClipboard{paste: []string{"sh", "-c", `printf %s "$WAYLAND_DISPLAY"`}, env: []string{"WAYLAND_DISPLAY=wayland-9"}}
	if got, err := cb.Read(); err != nil || got != "wayland-9" {
		t.Errorf("Read = %q, %v", got, err)
	}
}
