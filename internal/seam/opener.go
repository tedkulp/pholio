package seam

import (
	"os/exec"
	"runtime"
)

// SystemOpener is the real Opener: it hands the URL to xdg-open, or to open
// on macOS, and does not wait for the handler to finish.
type SystemOpener struct{}

// Open starts the platform's URL handler on url.
func (SystemOpener) Open(url string) error {
	name := "xdg-open"
	if runtime.GOOS == "darwin" {
		name = "open"
	}
	cmd := exec.Command(name, url) //nolint:gosec // the URL is an argument, not a shell command
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
