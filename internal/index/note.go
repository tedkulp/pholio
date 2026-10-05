// Package index keeps the in-memory Vault index: every Note's filename,
// title, headings, Links and Tasks. It answers Task List, Link resolution and
// Backlink queries, and takes updates from open buffers.
package index

import (
	"path"
	"strings"
)

// Note is the parsed form of one Note.
type Note struct {
	// Path is Vault-relative and slash-separated, e.g. "zettels/idea.md".
	Path string
	// Name is the filename without ".md".
	Name string
	// Title is the text of the first "# " heading, or "".
	Title string
	// Contents is the whole file.
	Contents string
	Headings []Heading
	Links    []Link
	Tasks    []Task
	// Conflict marks a Syncthing "*.sync-conflict-*.md" file. Conflict
	// Notes are left out of Tasks and Link resolution.
	Conflict bool
}

// Heading is one ATX heading.
type Heading struct {
	Level int
	Text  string
	// Line is 0-based.
	Line int
}

// Parse parses the contents of the Note at the Vault-relative path p.
// Headings, Links and Tasks inside fenced code blocks are skipped.
func Parse(p string, contents []byte) Note {
	base := path.Base(p)
	n := Note{
		Path:     p,
		Name:     strings.TrimSuffix(base, ".md"),
		Contents: string(contents),
		Conflict: strings.Contains(base, ".sync-conflict-"),
	}
	var fences Fences
	s := n.Contents
	for lineNo := 0; s != ""; lineNo++ {
		var line string
		if i := strings.IndexByte(s, '\n'); i >= 0 {
			line, s = s[:i], s[i+1:]
		} else {
			line, s = s, ""
		}
		line = strings.TrimSuffix(line, "\r")

		if fences.Code(line) {
			continue
		}
		if h, ok := parseHeading(line); ok {
			h.Line = lineNo
			n.Headings = append(n.Headings, h)
			if h.Level == 1 && n.Title == "" {
				n.Title = h.Text
			}
		} else if t, ok := ParseTask(line); ok {
			t.Path, t.Line = p, lineNo
			n.Tasks = append(n.Tasks, t)
		}
		n.Links = appendLinks(n.Links, line, lineNo)
	}
	return n
}

// Fences follows fenced code blocks through a Note, line by line.
type Fences struct {
	open string // the opening fence while inside a code block
}

// Code reports whether line, the next line of the Note, is fenced code: a
// fence line or a line inside a block. Lines must be fed in order from the
// first, without their newline.
func (f *Fences) Code(line string) bool {
	line = strings.TrimSuffix(line, "\r")
	if fe := fenceOf(line); fe != "" {
		switch {
		case f.open == "":
			f.open = fe
			return true
		case fe[0] == f.open[0] && len(fe) >= len(f.open) && strings.TrimSpace(strings.TrimLeft(line, " ")[len(fe):]) == "":
			f.open = ""
			return true
		}
	}
	return f.open != ""
}

// fenceOf returns the run of ``` or ~~~ (3 or more) that opens line after up
// to 3 spaces of indent, or "".
func fenceOf(line string) string {
	t := strings.TrimLeft(line, " ")
	if len(line)-len(t) > 3 || len(t) < 3 || (t[0] != '`' && t[0] != '~') {
		return ""
	}
	i := 0
	for i < len(t) && t[i] == t[0] {
		i++
	}
	if i < 3 {
		return ""
	}
	return t[:i]
}

// parseHeading reads an ATX heading: up to 3 spaces, 1-6 '#', then a space
// or end of line. A closing run of '#' is dropped.
func parseHeading(line string) (Heading, bool) {
	t := strings.TrimLeft(line, " ")
	if len(line)-len(t) > 3 {
		return Heading{}, false
	}
	level := 0
	for level < len(t) && t[level] == '#' {
		level++
	}
	if level == 0 || level > 6 || (level < len(t) && t[level] != ' ' && t[level] != '\t') {
		return Heading{}, false
	}
	text := strings.TrimSpace(t[level:])
	if closed := strings.TrimRight(text, "#"); closed != text && (closed == "" || strings.HasSuffix(closed, " ")) {
		text = strings.TrimSpace(closed)
	}
	return Heading{Level: level, Text: text}, true
}
