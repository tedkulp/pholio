package engine

import "strings"

// Clipboard is the system clipboard. It is a seam.Clipboard, so the app
// passes its own.
type Clipboard interface {
	Read() (string, error)
	Write(text string) error
}

// clipReg reports whether reg is one of the registers that are the
// clipboard: unnamed, "+ and "*.
func clipReg(reg rune) bool { return reg == '"' || reg == '+' || reg == '*' }

// toClipboard writes r to the clipboard. A failed write is ignored: the
// registers still hold the text.
func (e *Engine) toClipboard(r register) {
	if e.Clipboard != nil {
		_ = e.Clipboard.Write(r.text)
	}
}

// fromClipboard is the register a put from the clipboard uses. Text that
// matches the unnamed register is pholio's own, so the register is used
// and keeps its shape (lines or a block). Other text is pasted as lines
// when it ends in a newline, else inline. Without a readable, non-empty
// clipboard it falls back to the unnamed register.
func (e *Engine) fromClipboard() (register, bool) {
	r, ok := e.regs['"']
	if e.Clipboard == nil {
		return r, ok
	}
	text, err := e.Clipboard.Read()
	if err != nil || text == "" || (ok && text == r.text) {
		return r, ok
	}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	return register{text: text, linewise: strings.HasSuffix(text, "\n")}, true
}
