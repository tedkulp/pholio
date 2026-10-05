package index

import (
	"cmp"
	"path"
	"slices"
	"strings"
)

// Resolution is where a Link points.
type Resolution struct {
	// Path is the Vault-relative path of the target Note.
	Path string
	// Ambiguous is set when more than one Note matched. Path is then the
	// shortest of Candidates, and the UI shows a warning.
	Ambiguous bool
	// Candidates lists every match, shortest first, when Ambiguous.
	Candidates []string
}

// Resolve finds the Note that l, found in the Note at from, points to. ok is
// false for a dangling Link. Conflict Notes are never targets.
//
// A WikiLink matches a filename without ".md" anywhere in the Vault, ignoring
// case; "folder/name" matches only Notes whose path ends in that folder. A
// MarkdownLink is a path relative to from. A Link with no Target points at
// from itself.
func (ix *Index) Resolve(from string, l Link) (Resolution, bool) {
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	return ix.resolve(from, l)
}

func (ix *Index) resolve(from string, l Link) (Resolution, bool) {
	if l.Target == "" {
		return Resolution{Path: from}, true
	}
	if l.Kind == MarkdownLink {
		p := path.Join(path.Dir(from), l.Target)
		if n, ok := ix.notes[p]; ok && !n.Conflict && !strings.HasPrefix(p, "../") {
			return Resolution{Path: p}, true
		}
		return Resolution{}, false
	}
	return ix.resolveName(l.Target)
}

// resolveName resolves a WikiLink target. Callers hold mu.
func (ix *Index) resolveName(target string) (Resolution, bool) {
	return ResolveName(target, func(name string) []string { return ix.byName[name] })
}

// ResolveName resolves a WikiLink target the way Resolve does, among the
// Notes named returns for a lower-cased filename without ".md". It lets
// code holding its own list of Notes (a planned rename) resolve Links by
// the same rules.
func ResolveName(target string, named func(name string) []string) (Resolution, bool) {
	target = strings.TrimSuffix(strings.Trim(target, "/"), ".md")
	folder, name := "", target
	if i := strings.LastIndexByte(target, '/'); i >= 0 {
		folder, name = strings.ToLower(target[:i]), target[i+1:]
	}
	var matches []string
	for _, p := range named(strings.ToLower(name)) {
		if folder != "" {
			dir := strings.ToLower(path.Dir(p))
			if dir != folder && !strings.HasSuffix(dir, "/"+folder) {
				continue
			}
		}
		matches = append(matches, p)
	}
	switch len(matches) {
	case 0:
		return Resolution{}, false
	case 1:
		return Resolution{Path: matches[0]}, true
	}
	slices.SortFunc(matches, shorterPath)
	return Resolution{Path: matches[0], Ambiguous: true, Candidates: matches}, true
}

// shorterPath orders paths by folder depth, then length, then bytes.
func shorterPath(a, b string) int {
	return cmp.Or(
		cmp.Compare(strings.Count(a, "/"), strings.Count(b, "/")),
		cmp.Compare(len(a), len(b)),
		strings.Compare(a, b),
	)
}

// Backlink is one Link pointing at a Note.
type Backlink struct {
	// Source is the Vault-relative path of the linking Note.
	Source string
	Link   Link
	// Context is the line the Link is on.
	Context string
}

// Backlinks returns the Links that resolve to the Note at p, sorted by
// source path and position. Links within p to its own headings and Links
// from Conflict Notes are left out.
func (ix *Index) Backlinks(p string) []Backlink {
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	var out []Backlink
	for src, n := range ix.notes {
		if n.Conflict {
			continue
		}
		for _, l := range n.Links {
			if l.Target == "" {
				continue
			}
			if r, ok := ix.resolve(src, l); ok && r.Path == p {
				out = append(out, Backlink{Source: src, Link: l, Context: lineAt(n.Contents, l.Line)})
			}
		}
	}
	slices.SortFunc(out, func(a, b Backlink) int {
		return cmp.Or(
			strings.Compare(a.Source, b.Source),
			cmp.Compare(a.Link.Line, b.Link.Line),
			cmp.Compare(a.Link.Start, b.Link.Start),
		)
	})
	return out
}

// lineAt returns the 0-based line n of s, without its line ending.
func lineAt(s string, n int) string {
	for ; n > 0; n-- {
		i := strings.IndexByte(s, '\n')
		if i < 0 {
			return ""
		}
		s = s[i+1:]
	}
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSuffix(s, "\r")
}
