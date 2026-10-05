// PROTOTYPE — throwaway code for wayfinder ticket "Vim editor prototype".
// Not production. See ../README.md.

package engine

import (
	"slices"
	"strings"
)

// Pos is a position in the buffer. Col is a byte offset into the line,
// always on a grapheme-cluster boundary.
type Pos struct{ Line, Col int }

func (a Pos) Less(b Pos) bool { return a.Line < b.Line || a.Line == b.Line && a.Col < b.Col }

func order(a, b Pos) (Pos, Pos) {
	if b.Less(a) {
		return b, a
	}
	return a, b
}

// Buffer is a plain slice of lines (no trailing "\n" stored).
// Only two primitives mutate it: insert and delete. Everything else
// (operators, undo, paste) is built on those two.
type Buffer struct{ lines []string }

func NewBuffer(text string) *Buffer {
	text = strings.TrimSuffix(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	return &Buffer{lines: strings.Split(text, "\n")}
}

func (b *Buffer) String() string    { return strings.Join(b.lines, "\n") + "\n" }
func (b *Buffer) Line(i int) string { return b.lines[i] }
func (b *Buffer) LineCount() int    { return len(b.lines) }

// insert puts s at p and returns the position just after it.
func (b *Buffer) insert(p Pos, s string) Pos {
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
	text := b.slice(a, z)
	head, tail := b.lines[a.Line][:a.Col], b.lines[z.Line][z.Col:]
	b.lines = slices.Replace(b.lines, a.Line, z.Line+1, head+tail)
	return text
}

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

// endOf returns where inserting s at p would leave the cursor.
func endOf(p Pos, s string) Pos {
	n := strings.Count(s, "\n")
	if n == 0 {
		return Pos{p.Line, p.Col + len(s)}
	}
	return Pos{p.Line + n, len(s) - strings.LastIndex(s, "\n") - 1}
}

// offset/posAt convert between Pos and a flat byte offset into String().
// O(lines); fine at note size, used by search and bracket objects.
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
