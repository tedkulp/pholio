package engine

import (
	"slices"
	"strings"
	"sync/atomic"
)

// Pos is a position in the buffer. Col is a byte offset into the line and
// always sits on a grapheme-cluster boundary.
type Pos struct{ Line, Col int }

// Less reports whether a comes before b.
func (a Pos) Less(b Pos) bool { return a.Line < b.Line || a.Line == b.Line && a.Col < b.Col }

func order(a, b Pos) (Pos, Pos) {
	if b.Less(a) {
		return b, a
	}
	return a, b
}

// Buffer is a plain slice of lines, stored without their "\n".
// Only insert and delete mutate it; operators, undo and paste are built on
// those two.
type Buffer struct {
	lines   []string
	version uint64
}

// versions hands out buffer versions. It is shared by every Buffer, so a
// version never repeats, even across buffers.
var versions atomic.Uint64

// NewBuffer splits text into lines. CRLF becomes LF and one trailing newline
// is dropped, so String gives the text back with a single final "\n".
func NewBuffer(text string) *Buffer {
	text = strings.TrimSuffix(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	return &Buffer{lines: strings.Split(text, "\n"), version: versions.Add(1)}
}

// Version identifies the buffer's contents: it changes with every edit and
// is unique across buffers, so a host can tell cheaply whether the text
// changed since it last looked.
func (b *Buffer) Version() uint64 { return b.version }

// String returns the whole buffer. It always ends in "\n".
func (b *Buffer) String() string { return strings.Join(b.lines, "\n") + "\n" }

// Line returns line i without its newline.
func (b *Buffer) Line(i int) string { return b.lines[i] }

// LineCount is the number of lines. It is never less than 1.
func (b *Buffer) LineCount() int { return len(b.lines) }

// insert puts s at p and returns the position just after it.
func (b *Buffer) insert(p Pos, s string) Pos {
	b.version = versions.Add(1)
	line := b.lines[p.Line]
	head, tail := line[:p.Col], line[p.Col:]
	parts := strings.Split(s, "\n")
	if len(parts) == 1 {
		b.lines[p.Line] = head + s + tail
		return Pos{p.Line, p.Col + len(s)}
	}
	last := len(parts) - 1
	end := Pos{p.Line + last, len(parts[last])}
	parts[0] = head + parts[0]
	parts[last] += tail
	b.lines = slices.Replace(b.lines, p.Line, p.Line+1, parts...)
	return end
}

// delete removes [a, z) and returns the removed text.
func (b *Buffer) delete(a, z Pos) string {
	b.version = versions.Add(1)
	text := b.slice(a, z)
	head, tail := b.lines[a.Line][:a.Col], b.lines[z.Line][z.Col:]
	b.lines = slices.Replace(b.lines, a.Line, z.Line+1, head+tail)
	return text
}

// slice returns the text in [a, z).
func (b *Buffer) slice(a, z Pos) string {
	if a.Line == z.Line {
		return b.lines[a.Line][a.Col:z.Col]
	}
	var sb strings.Builder
	sb.WriteString(b.lines[a.Line][a.Col:])
	for i := a.Line + 1; i < z.Line; i++ {
		sb.WriteString("\n" + b.lines[i])
	}
	sb.WriteString("\n" + b.lines[z.Line][:z.Col])
	return sb.String()
}

// endOf returns the position just after s if s were inserted at p.
func endOf(p Pos, s string) Pos {
	n := strings.Count(s, "\n")
	if n == 0 {
		return Pos{p.Line, p.Col + len(s)}
	}
	return Pos{p.Line + n, len(s) - strings.LastIndex(s, "\n") - 1}
}

// offset and posAt convert between a Pos and a byte offset into String().
// Both are O(lines): fine at Note size, and only used by bracket matching.
func (b *Buffer) offset(p Pos) int {
	o := 0
	for i := 0; i < p.Line; i++ {
		o += len(b.lines[i]) + 1
	}
	return o + p.Col
}

func (b *Buffer) posAt(o int) Pos {
	for i, l := range b.lines {
		if o <= len(l) {
			return Pos{i, o}
		}
		o -= len(l) + 1
	}
	last := len(b.lines) - 1
	return Pos{last, len(b.lines[last])}
}
