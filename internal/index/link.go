package index

import (
	"net/url"
	"strings"
)

// LinkKind tells the two Link syntaxes apart.
type LinkKind uint8

// The Link syntaxes.
const (
	// WikiLink is [[target#Heading|alias]].
	WikiLink LinkKind = iota
	// MarkdownLink is [alias](target.md#Heading), a path relative to the
	// Note it is in.
	MarkdownLink
)

// Link is one Link found in a Note.
type Link struct {
	Kind LinkKind
	// Target is the name ("name", "folder/name") of a WikiLink, or the
	// unescaped relative path of a MarkdownLink. It is "" for a Link to a
	// heading in the same Note ("[[#Heading]]").
	Target string
	// Heading is the part after '#', or "".
	Heading string
	// Alias is the part after '|' of a WikiLink, or the text of a
	// MarkdownLink.
	Alias string
	// Line is 0-based. Start and End are byte offsets into the line that
	// cover the whole Link, from "[" to the closing "]]" or ")".
	Line, Start, End int
}

// appendLinks adds the Links on one line (outside fenced code) to links.
// Inline code spans are skipped.
func appendLinks(links []Link, line string, lineNo int) []Link {
	if strings.IndexByte(line, '[') < 0 {
		return links
	}
	for i := 0; i < len(line); {
		switch line[i] {
		case '`':
			// Skip an inline code span: a run of backticks up to the
			// matching run.
			j := i
			for j < len(line) && line[j] == '`' {
				j++
			}
			ticks := line[i:j]
			if end := strings.Index(line[j:], ticks); end >= 0 {
				i = j + end + len(ticks)
			} else {
				i = j
			}
		case '[':
			if l, end, ok := wikiAt(line, i); ok {
				l.Line = lineNo
				links = append(links, l)
				i = end
			} else if l, end, ok := markdownAt(line, i); ok {
				l.Line = lineNo
				links = append(links, l)
				i = end
			} else {
				i++
			}
		default:
			i++
		}
	}
	return links
}

// wikiAt parses a [[...]] Link starting at line[i].
func wikiAt(line string, i int) (Link, int, bool) {
	if !strings.HasPrefix(line[i:], "[[") {
		return Link{}, 0, false
	}
	body := line[i+2:]
	end := strings.Index(body, "]]")
	if end < 0 {
		return Link{}, 0, false
	}
	body = body[:end]
	if strings.ContainsAny(body, "[]") || strings.TrimSpace(body) == "" {
		return Link{}, 0, false
	}
	l := Link{Kind: WikiLink, Start: i, End: i + 2 + end + 2}
	body, l.Alias, _ = strings.Cut(body, "|")
	body, l.Heading, _ = strings.Cut(body, "#")
	l.Target = strings.TrimSpace(body)
	l.Heading = strings.TrimSpace(l.Heading)
	l.Alias = strings.TrimSpace(l.Alias)
	return l, l.End, true
}

// markdownAt parses an [alias](path.md#Heading) Link starting at line[i].
// Only targets ending in ".md" (before any #Heading) count; URLs don't.
func markdownAt(line string, i int) (Link, int, bool) {
	textEnd := strings.IndexByte(line[i+1:], ']')
	if textEnd < 0 {
		return Link{}, 0, false
	}
	textEnd += i + 1
	if textEnd+1 >= len(line) || line[textEnd+1] != '(' {
		return Link{}, 0, false
	}
	dest := line[textEnd+2:]
	closeAt := strings.IndexByte(dest, ')')
	if closeAt < 0 {
		return Link{}, 0, false
	}
	dest = strings.TrimSpace(dest[:closeAt])
	dest = strings.TrimSuffix(strings.TrimPrefix(dest, "<"), ">")
	if strings.Contains(dest, "://") {
		return Link{}, 0, false
	}
	target, heading, _ := strings.Cut(dest, "#")
	if !IsNote(target) {
		return Link{}, 0, false
	}
	if u, err := url.PathUnescape(target); err == nil {
		target = u
	}
	l := Link{
		Kind:    MarkdownLink,
		Target:  target,
		Heading: heading,
		Alias:   line[i+1 : textEnd],
		Start:   i,
		End:     textEnd + 2 + closeAt + 1,
	}
	return l, l.End, true
}
