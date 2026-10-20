package seam

import (
	"context"
	"errors"
	"os/exec"
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
}

var _ Clipboard = SystemClipboard{}

// NewSystemClipboard returns the clipboard of the platform and display
// that getenv describes, using the first tool found on $PATH.
func NewSystemClipboard(getenv func(string) string) SystemClipboard {
	return pickClipboard(runtime.GOOS, getenv, exec.LookPath)
}

func pickClipboard(goos string, getenv func(string) string, lookPath func(string) (string, error)) SystemClipboard {
	var tools []SystemClipboard
	switch {
	case goos == "darwin":
		tools = append(tools, SystemClipboard{[]string{"pbcopy"}, []string{"pbpaste"}})
	case getenv("WAYLAND_DISPLAY") != "":
		tools = append(tools, SystemClipboard{[]string{"wl-copy"}, []string{"wl-paste", "--no-newline"}})
	}
	if getenv("DISPLAY") != "" {
		tools = append(tools,
			SystemClipboard{[]string{"xclip", "-selection", "clipboard", "-in"}, []string{"xclip", "-selection", "clipboard", "-out"}},
			SystemClipboard{[]string{"xsel", "--clipboard", "--input"}, []string{"xsel", "--clipboard", "--output"}},
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
	out, err := exec.CommandContext(ctx, c.paste[0], c.paste[1:]...).Output() //nolint:gosec // a fixed clipboard tool
	return string(out), err
}

// Write puts text on the clipboard.
func (c SystemClipboard) Write(text string) error {
	if c.copy == nil {
		return ErrNoClipboard
	}
	ctx, cancel := context.WithTimeout(context.Background(), clipTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.copy[0], c.copy[1:]...) //nolint:gosec // a fixed clipboard tool
	cmd.Stdin = strings.NewReader(text)
	// No stdout or stderr: wl-copy and xclip leave a child serving the
	// selection, and a pipe it inherits would keep Run waiting for it.
	return cmd.Run()
}
