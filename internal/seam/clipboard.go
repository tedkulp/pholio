package seam

import (
	"context"
	"errors"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ErrNoClipboard is returned by a SystemClipboard that found no clipboard
// tool, as over SSH.
var ErrNoClipboard = errors.New("no clipboard tool")

// clipTimeout bounds each clipboard tool run, so a stuck selection owner
// can't hang the editor.
const clipTimeout = time.Second

// SystemClipboard is the real Clipboard: it runs the platform's clipboard
// tools (pbcopy/pbpaste on macOS, wl-copy/wl-paste on Wayland, xclip or
// xsel on X11). The zero value has no tools and returns ErrNoClipboard.
type SystemClipboard struct {
	copy, paste []string // the commands that write and read the clipboard
	env         []string // added to the tools' environment, as "KEY=value"
}

var _ Clipboard = SystemClipboard{}

// NewSystemClipboard returns the clipboard of the platform and display
// that getenv describes, using the first tool found on $PATH. With no
// display set, as in a multiplexer pane started outside the desktop, it
// uses the Wayland display found by fallbackDisplay.
func NewSystemClipboard(getenv func(string) string) SystemClipboard {
	d := fallbackDisplay(getenv)
	if d == "" {
		return pickClipboard(runtime.GOOS, getenv, exec.LookPath)
	}
	c := pickClipboard(runtime.GOOS, func(k string) string {
		if k == "WAYLAND_DISPLAY" {
			return d
		}
		return getenv(k)
	}, exec.LookPath)
	c.env = []string{"WAYLAND_DISPLAY=" + d}
	return c
}

// fallbackDisplay is the Wayland display to use when neither
// WAYLAND_DISPLAY nor DISPLAY is set: the one wayland-* socket in
// $XDG_RUNTIME_DIR that accepts a connection. With none, or several (a
// nested compositor, say), it returns "" rather than guess, since copies
// sent to the wrong compositor would vanish without an error.
func fallbackDisplay(getenv func(string) string) string {
	dir := getenv("XDG_RUNTIME_DIR")
	if getenv("WAYLAND_DISPLAY") != "" || getenv("DISPLAY") != "" || dir == "" {
		return ""
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var live []string
	for _, ent := range entries {
		name := ent.Name()
		if !strings.HasPrefix(name, "wayland-") || ent.Type() != fs.ModeSocket {
			continue
		}
		conn, err := net.DialTimeout("unix", filepath.Join(dir, name), 100*time.Millisecond)
		if err != nil {
			continue // stale: the compositor is gone
		}
		_ = conn.Close()
		live = append(live, name)
	}
	if len(live) != 1 {
		return ""
	}
	return live[0]
}

func pickClipboard(goos string, getenv func(string) string, lookPath func(string) (string, error)) SystemClipboard {
	var tools []SystemClipboard
	switch {
	case goos == "darwin":
		tools = append(tools, SystemClipboard{copy: []string{"pbcopy"}, paste: []string{"pbpaste"}})
	case getenv("WAYLAND_DISPLAY") != "":
		tools = append(tools, SystemClipboard{copy: []string{"wl-copy"}, paste: []string{"wl-paste", "--no-newline"}})
	}
	if getenv("DISPLAY") != "" {
		tools = append(tools,
			SystemClipboard{copy: []string{"xclip", "-selection", "clipboard", "-in"}, paste: []string{"xclip", "-selection", "clipboard", "-out"}},
			SystemClipboard{copy: []string{"xsel", "--clipboard", "--input"}, paste: []string{"xsel", "--clipboard", "--output"}},
		)
	}
	for _, t := range tools {
		if _, err := lookPath(t.copy[0]); err == nil {
			return t
		}
	}
	return SystemClipboard{}
}

// Read returns the clipboard text.
func (c SystemClipboard) Read() (string, error) {
	if c.paste == nil {
		return "", ErrNoClipboard
	}
	ctx, cancel := context.WithTimeout(context.Background(), clipTimeout)
	defer cancel()
	out, err := c.command(ctx, c.paste).Output()
	return string(out), err
}

// Write puts text on the clipboard.
func (c SystemClipboard) Write(text string) error {
	if c.copy == nil {
		return ErrNoClipboard
	}
	ctx, cancel := context.WithTimeout(context.Background(), clipTimeout)
	defer cancel()
	cmd := c.command(ctx, c.copy)
	cmd.Stdin = strings.NewReader(text)
	// No stdout or stderr: wl-copy and xclip leave a child serving the
	// selection, and a pipe it inherits would keep Run waiting for it.
	return cmd.Run()
}

// command makes the command for a clipboard tool, with c.env added.
func (c SystemClipboard) command(ctx context.Context, args []string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, args[0], args[1:]...) //nolint:gosec // a fixed clipboard tool
	if c.env != nil {
		cmd.Env = append(os.Environ(), c.env...)
	}
	return cmd
}
