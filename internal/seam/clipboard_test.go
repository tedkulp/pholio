package seam

import (
	"errors"
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
