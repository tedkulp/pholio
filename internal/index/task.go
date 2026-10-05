package index

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Status is a Task Status.
type Status uint8

// The Task Statuses. InProgress still counts as open.
const (
	Open Status = iota
	InProgress
	Done
	Cancelled
)

// IsOpen reports whether s counts as open (Open or InProgress).
func (s Status) IsOpen() bool { return s == Open || s == InProgress }

// Task is one markdown checkbox line and its Task Metadata.
type Task struct {
	// Path is the Vault-relative, slash-separated path of the Note. It is
	// empty for a Task from ParseTask.
	Path string
	// Line is the 0-based line number in the Note.
	Line int
	// Indent is the count of leading whitespace bytes before the list marker.
	Indent int
	Status Status
	// Mark is the character between the brackets.
	Mark rune
	// Text is everything after the checkbox, metadata and tags included.
	Text string
	// Summary is Text with the key:value and #tag tokens removed.
	Summary string
	// Meta holds the key:value pairs; nil when there are none.
	Meta map[string]string
	// Tags holds the #tags without the '#'; nil when there are none.
	Tags []string
}

// ParseTask parses one line as a Task. ok is false when the line isn't one.
// Path and Line are left zero.
func ParseTask(line string) (t Task, ok bool) {
	rest := strings.TrimLeft(line, " \t")
	t.Indent = len(line) - len(rest)
	rest, ok = cutListMarker(rest)
	if !ok {
		return Task{}, false
	}
	// "[c]" with exactly one rune inside, then a space or end of line.
	if !strings.HasPrefix(rest, "[") {
		return Task{}, false
	}
	mark, n := utf8.DecodeRuneInString(rest[1:])
	if n == 0 || len(rest) < 2+n || rest[1+n] != ']' {
		return Task{}, false
	}
	rest = rest[2+n:]
	if rest != "" && rest[0] != ' ' && rest[0] != '\t' {
		return Task{}, false
	}
	t.Mark = mark
	t.Status = statusOf(mark)
	t.Text = strings.TrimSpace(rest)
	parseMeta(&t)
	return t, true
}

// cutListMarker removes a "-", "*", "+", "1." or "1)" marker and the space
// after it.
func cutListMarker(s string) (string, bool) {
	if s == "" {
		return s, false
	}
	switch s[0] {
	case '-', '*', '+':
		s = s[1:]
	default:
		i := 0
		for i < len(s) && i < 9 && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		if i == 0 || i >= len(s) || (s[i] != '.' && s[i] != ')') {
			return s, false
		}
		s = s[i+1:]
	}
	if s == "" || (s[0] != ' ' && s[0] != '\t') {
		return s, false
	}
	return strings.TrimLeft(s, " \t"), true
}

func statusOf(mark rune) Status {
	switch mark {
	case '/':
		return InProgress
	case 'x', 'X':
		return Done
	case '-':
		return Cancelled
	}
	return Open
}

func parseMeta(t *Task) {
	words := strings.Fields(t.Text)
	kept := words[:0:0]
	for _, w := range words {
		if tag, ok := parseTag(w); ok {
			t.Tags = append(t.Tags, tag)
			continue
		}
		if k, v, ok := parseKeyValue(w); ok {
			if t.Meta == nil {
				t.Meta = map[string]string{}
			}
			t.Meta[k] = v
			continue
		}
		kept = append(kept, w)
	}
	t.Summary = strings.Join(kept, " ")
}

// parseTag accepts "#name" where name is letters, digits, '_', '-' or '/',
// and is not all digits (so "#12" stays text).
func parseTag(w string) (string, bool) {
	if len(w) < 2 || w[0] != '#' {
		return "", false
	}
	name := w[1:]
	digits := true
	for _, r := range name {
		if !isTagRune(r) {
			return "", false
		}
		if !unicode.IsDigit(r) {
			digits = false
		}
	}
	return name, !digits
}

func isTagRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' || r == '/'
}

// parseKeyValue accepts "key:value" where key starts with a letter and holds
// letters, digits, '_' or '-'. A value starting with "/" is left alone so
// URLs such as https://x.io stay text.
func parseKeyValue(w string) (k, v string, ok bool) {
	k, v, found := strings.Cut(w, ":")
	if !found || k == "" || v == "" || v[0] == '/' {
		return "", "", false
	}
	for i, r := range k {
		if i == 0 && !unicode.IsLetter(r) {
			return "", "", false
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-' {
			return "", "", false
		}
	}
	return k, v, true
}
