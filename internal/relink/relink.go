// Package relink plans the Link rewrites that renaming or moving Notes
// needs. It is pure: it takes the parsed Notes of the Vault and the moves,
// and returns each Note's new contents, leaving the reading and writing to
// the caller.
package relink

import (
	"path"
	"slices"
	"strings"

	"github.com/tedkulp/pholio/internal/index"
)

// Edit is the rewrite of one Note.
type Edit struct {
	// Path is the Note's Vault-relative path before the move, and To its
	// path after it (the same unless the Note itself moves).
	Path, To string
	// Contents is the Note's text with its Links rewritten.
	Contents string
	// Links is how many Links changed.
	Links int
}

// change replaces bytes [start, end) of a line with text.
type change struct {
	line, start, end int
	text             string
}

// vault is the set of Notes on one side of the move, for resolving Links.
type vault struct {
	notes  map[string]bool
	byName map[string][]string // lower-cased name → paths
}

func newVault(paths []string) vault {
	v := vault{notes: map[string]bool{}, byName: map[string][]string{}}
	for _, p := range paths {
		v.notes[p] = true
		k := strings.ToLower(index.NoteName(p))
		v.byName[k] = append(v.byName[k], p)
	}
	return v
}

// resolve finds where l, written in the Note at from, points in v.
func (v vault) resolve(from string, l index.Link) (index.Resolution, bool) {
	if l.Kind == index.MarkdownLink {
		p := path.Join(path.Dir(from), l.Target)
		if v.notes[p] {
			return index.Resolution{Path: p}, true
		}
		return index.Resolution{}, false
	}
	return index.ResolveName(l.Target, func(name string) []string { return v.byName[name] })
}

// Plan works out the Link rewrites for moves, a map of Vault-relative Note
// paths before → after (every Note under a moved folder is listed). A Link
// is rewritten when its text would no longer lead to the Note it led to
// before: its target moved, the Note it is in moved (relative Markdown
// Links), or a moved Note took over its name. New WikiLink text is the
// shortest that resolves to exactly that Note. #heading and |alias parts
// are kept. Conflict Notes, Links in code and dangling Links are left
// alone. Edits come sorted by Path.
func Plan(notes []index.Note, moves map[string]string) []Edit {
	moved := func(p string) string {
		if to, ok := moves[p]; ok {
			return to
		}
		return p
	}
	var before, after []string
	for _, n := range notes {
		if n.Conflict {
			continue
		}
		before = append(before, n.Path)
		after = append(after, moved(n.Path))
	}
	old, now := newVault(before), newVault(after)

	var edits []Edit
	for _, n := range notes {
		if n.Conflict {
			continue
		}
		from, to := n.Path, moved(n.Path)
		var changes []change
		for _, l := range n.Links {
			if l.Target == "" {
				continue
			}
			was, ok := old.resolve(from, l)
			if !ok {
				continue
			}
			target := moved(was.Path)
			if is, ok := now.resolve(to, l); ok && is.Path == target && (was.Ambiguous || !is.Ambiguous) {
				continue
			}
			changes = append(changes, change{l.Line, l.Start, l.End, now.text(to, target, l)})
		}
		if len(changes) > 0 {
			edits = append(edits, Edit{Path: from, To: to, Contents: apply(n.Contents, changes), Links: len(changes)})
		}
	}
	slices.SortFunc(edits, func(a, b Edit) int { return strings.Compare(a.Path, b.Path) })
	return edits
}

// text is the new text of Link l in the Note at from, pointing to target.
func (v vault) text(from, target string, l index.Link) string {
	if l.Kind == index.MarkdownLink {
		dest := escape(relative(path.Dir(from), target))
		if l.Heading != "" {
			dest += "#" + l.Heading
		}
		return "[" + l.Alias + "](" + dest + ")"
	}
	s := "[[" + v.shortest(target)
	if l.Heading != "" {
		s += "#" + l.Heading
	}
	if l.Alias != "" {
		s += "|" + l.Alias
	}
	return s + "]]"
}

// shortest is the shortest WikiLink target that resolves to exactly the
// Note at p: its name, then with as many of its folders as it takes. The
// full path is the fallback.
func (v vault) shortest(p string) string {
	full := index.TrimNoteExt(p)
	segs := strings.Split(full, "/")
	for i := len(segs) - 1; i >= 0; i-- {
		cand := strings.Join(segs[i:], "/")
		if r, ok := index.ResolveName(cand, func(name string) []string { return v.byName[name] }); ok && !r.Ambiguous && r.Path == p {
			return cand
		}
	}
	return full
}

// relative is the slash path from folder dir to p.
func relative(dir, p string) string {
	if dir == "." {
		return p
	}
	from, to := strings.Split(dir, "/"), strings.Split(p, "/")
	i := 0
	for i < len(from) && i < len(to)-1 && from[i] == to[i] {
		i++
	}
	return strings.Repeat("../", len(from)-i) + strings.Join(to[i:], "/")
}

// escape percent-encodes what would end or break a Markdown Link target.
func escape(p string) string {
	return strings.NewReplacer("%", "%25", " ", "%20", "(", "%28", ")", "%29", "#", "%23").Replace(p)
}

// apply makes changes to s, a Note's text. Changes on a line must not
// overlap; they come in Link order.
func apply(s string, changes []change) string {
	lines := strings.Split(s, "\n")
	for i := len(changes) - 1; i >= 0; i-- {
		c := changes[i]
		l := lines[c.line]
		lines[c.line] = l[:c.start] + c.text + l[c.end:]
	}
	return strings.Join(lines, "\n")
}
