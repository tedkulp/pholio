package engine

import "strings"

// Reload replaces the buffer with text read back from disk, after the file
// changed outside pholio. It is one undo step, so u brings the old text
// back. The cursor keeps its line and column, clamped to the new text, the
// engine returns to normal mode and the buffer is clean.
func (e *Engine) Reload(text string) {
	e.replace(text)
	e.MarkSaved()
}

// Replace is Reload for text that is not what is on disk, such as Links
// rewritten into an unsaved buffer: the buffer is left dirty.
func (e *Engine) Replace(text string) {
	e.replace(text)
	e.Dirty = e.head() != e.saved
}

func (e *Engine) replace(text string) {
	e.commit()
	e.Mode, e.keys, e.recording, e.blockIns = Normal, nil, false, nil
	old := strings.TrimSuffix(e.Buf.String(), "\n")
	buf := NewBuffer(text)
	if s := strings.TrimSuffix(buf.String(), "\n"); s != old {
		c := change{before: e.Cur, edits: []edit{{at: Pos{}, del: old, ins: s}}}
		e.Buf = buf
		e.SetCursor(e.Cur)
		c.after = e.Cur
		e.push(c)
	}
	e.SetCursor(e.Cur)
}
