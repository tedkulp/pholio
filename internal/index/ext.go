package index

import (
	"path"
	"strings"
)

// noteExt is a Note's extension. It is matched ignoring case: Foo.MD is a
// Note too.
const noteExt = ".md"

// IsNote reports whether the file name or path p names a Note: it ends in
// ".md", in any case.
func IsNote(p string) bool {
	return len(p) >= len(noteExt) && strings.EqualFold(p[len(p)-len(noteExt):], noteExt)
}

// TrimNoteExt is p without its ".md", in any case; p unchanged when it has
// none.
func TrimNoteExt(p string) string {
	if IsNote(p) {
		return p[:len(p)-len(noteExt)]
	}
	return p
}

// NoteName is the Name of the Note at the slash path p: its file name
// without ".md".
func NoteName(p string) string { return TrimNoteExt(path.Base(p)) }
