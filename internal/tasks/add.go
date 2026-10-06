package tasks

import (
	"strings"

	"github.com/tedkulp/pholio/internal/index"
)

// AddAt is where the Task List's `a` puts a new Task in a Note's lines
// (without their newlines): after the last Task in the section under
// heading, right under heading when the section has no Task, or at the end
// when there is no such heading. The section runs to the next heading of
// the same or a higher level. Fenced code is skipped.
func AddAt(lines []string, heading string) int {
	heading = strings.TrimSpace(heading)
	level := headingLevel(heading)
	var f index.Fences
	at, in := -1, false
	for i, l := range lines {
		if f.Code(l) {
			continue
		}
		trimmed := strings.TrimSpace(l)
		if lv := headingLevel(trimmed); lv > 0 {
			if in && lv <= level {
				break
			}
			if !in && trimmed == heading {
				in, at = true, i+1
				continue
			}
		}
		if in {
			if _, ok := index.ParseTask(l); ok {
				at = i + 1
			}
		}
	}
	if at < 0 {
		return len(lines)
	}
	return at
}

// headingLevel is the number of #s opening an ATX heading, or 0.
func headingLevel(s string) int {
	n := len(s) - len(strings.TrimLeft(s, "#"))
	if n == 0 || n > 6 || (len(s) > n && s[n] != ' ' && s[n] != '\t') {
		return 0
	}
	return n
}

// AddFile puts line into the file at path where AddAt says, keeping the
// file's line endings, and writes it back.
func AddFile(fsys FS, path, heading, line string) error {
	data, err := fsys.ReadFile(path)
	if err != nil {
		return err
	}
	text := string(data)
	eol := "\n"
	if strings.Contains(text, "\r\n") {
		eol = "\r\n"
	}
	var lines []string
	if body := strings.TrimSuffix(strings.ReplaceAll(text, "\r\n", "\n"), "\n"); text != "" {
		lines = strings.Split(body, "\n")
	}
	at := AddAt(lines, heading)
	lines = append(lines[:at], append([]string{line}, lines[at:]...)...)
	return fsys.WriteFile(path, []byte(strings.Join(lines, eol)+eol))
}
