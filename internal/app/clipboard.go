package app

import (
	tea "charm.land/bubbletea/v2"

	"github.com/tedkulp/pholio/internal/seam"
)

// clipboard is the engines' clipboard: the system one, with OSC 52 for
// writes it can't take (no clipboard tool, as over SSH). OSC 52 can't be
// read back reliably, so reads still go to the system clipboard, and on
// failure the engine uses its own register. It is shared by Model copies.
type clipboard struct {
	sys     seam.Clipboard
	pending *string // text waiting to go out as OSC 52
}

func (c *clipboard) Read() (string, error) { return c.sys.Read() }

func (c *clipboard) Write(text string) error {
	err := c.sys.Write(text)
	if err != nil {
		c.pending = &text
	}
	return err
}

// osc52 takes the text waiting for OSC 52 and returns the command that
// sends it, or nil.
func (c *clipboard) osc52() tea.Cmd {
	if c == nil || c.pending == nil {
		return nil
	}
	text := *c.pending
	c.pending = nil
	return tea.SetClipboard(text)
}
