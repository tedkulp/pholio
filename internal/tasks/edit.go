package tasks

import (
	"strings"
	"time"

	"github.com/tedkulp/pholio/internal/index"
)

// Values are what the Task Editor edits on a Task line. Dates are ISO
// dates and Priority a level name ("highest" ... "lowest"); "" means the
// field is absent.
type Values struct {
	// Description is the Task's text without its Task Metadata fields and
	// its trailing block ID. #tags stay in it.
	Description string
	Status      index.Status
	Due         string
	Scheduled   string
	Start       string
	Priority    string
}

// editedKeys are the Task Metadata fields the Task Editor shows, in the
// order new ones are appended.
var editedKeys = []string{"due", "scheduled", "start", "priority"}

// get is v's value for an edited key.
func (v Values) get(key string) string {
	switch key {
	case "due":
		return v.Due
	case "scheduled":
		return v.Scheduled
	case "start":
		return v.Start
	case "priority":
		return v.Priority
	}
	return ""
}

// Read splits a Task line into the Task Editor's values. ok is false when
// line isn't a Task (an empty checkbox isn't one).
func Read(line string) (v Values, ok bool) {
	t, ok := index.ParseTask(line)
	if !ok || t.Text == "" {
		return Values{}, false
	}
	desc, fields, _ := split(line, t)
	v = Values{Description: desc, Status: t.Status}
	for _, f := range fields {
		switch f.Key {
		case "due", "scheduled", "start":
			if v.get(f.Key) == "" {
				v.set(f.Key, f.Value)
			}
		case "priority":
			if v.Priority == "" && f.Value != "" {
				v.Priority = f.Value
			}
		}
	}
	return v, true
}

// set sets v's date for key ("due", "scheduled" or "start").
func (v *Values) set(key, val string) {
	switch key {
	case "due":
		v.Due = val
	case "scheduled":
		v.Scheduled = val
	case "start":
		v.Start = val
	}
}

// Rewrite rebuilds a Task line from the Task Editor's values: the line's
// indent, list marker and checkbox, with the mark set from v.Status; then
// v.Description; then the line's Task Metadata fields in their order, each
// in its own Task Format, with edited values replaced and cleared ones
// dropped; then new fields, in the format of the line's first field, or
// def when it has none; then the block ID. The done date follows the
// Task Status change (see Stamp). ok is false, and line is returned, when it
// isn't a Task. An empty checkbox counts, so "- [ ]" builds a new Task.
func Rewrite(line string, v Values, def index.Format, today time.Time) (string, bool) {
	t, ok := index.ParseTask(line)
	if !ok {
		return line, false
	}
	_, fields, id := split(line, t)
	format := def
	if len(fields) > 0 {
		format = fields[0].Format
	}
	at := checkbox(line, t)
	mark := string(t.Mark)
	if t.Status != v.Status {
		mark = markOf[v.Status]
	}
	out := []string{line[:at] + mark + "]"}
	if d := strings.TrimSpace(v.Description); d != "" {
		out = append(out, d)
	}
	written := map[string]bool{}
	for _, f := range fields {
		val := v.get(f.Key)
		switch {
		case !isEdited(f.Key):
			out = append(out, f.Text)
		case val == "" || written[f.Key]:
		default:
			written[f.Key] = true
			out = append(out, f.with(val))
		}
	}
	for _, k := range editedKeys {
		if val := v.get(k); val != "" && !written[k] {
			out = append(out, newField(k, val, format))
		}
	}
	if id != "" {
		out = append(out, id)
	}
	return Stamp(line, strings.Join(out, " "), def, today), true
}

// markOf is the checkbox mark written for each Task Status.
var markOf = map[index.Status]string{
	index.Open: " ", index.InProgress: "/", index.Done: "x", index.Cancelled: "-",
}

// isEdited reports whether key is a field the Task Editor shows.
func isEdited(key string) bool {
	for _, k := range editedKeys {
		if k == key {
			return true
		}
	}
	return false
}

// field is one Task Metadata field of a line, with its text.
type field struct {
	index.Field
	Text string // the whole field as written
}

// with is f written with value val, in f's own Task Format and brackets.
func (f field) with(val string) string {
	if f.Key == "priority" && f.Format == index.Emoji {
		return priorityEmoji[val]
	}
	return f.Text[:f.ValStart-f.Start] + val + f.Text[f.ValEnd-f.Start:]
}

// priorityEmoji is each priority level's emoji.
var priorityEmoji = map[string]string{
	"highest": "🔺", "high": "⏫", "medium": "🔼", "low": "🔽", "lowest": "⏬",
}

// dateEmoji is the emoji written for each date the Task Editor adds.
var dateEmoji = map[string]string{"due": "📅", "scheduled": "⏳", "start": "🛫"}

// newField writes a field the line didn't have, in format.
func newField(key, val string, format index.Format) string {
	switch {
	case format == index.Dataview:
		return "[" + key + ":: " + val + "]"
	case key == "priority":
		return priorityEmoji[val]
	}
	return dateEmoji[key] + " " + val
}

// split cuts t's line (after the checkbox) into its description, its Task
// Metadata fields in order, and its trailing block ID ("^abc", or "").
func split(line string, t index.Task) (desc string, fields []field, id string) {
	text := line[checkbox(line, t)+len(string(t.Mark))+1:] // after "]"
	if loc := blockID.FindStringIndex(text); loc != nil {
		text, id = text[:loc[0]], strings.TrimSpace(text[loc[0]:])
	}
	var b strings.Builder
	at := 0
	for _, f := range index.Fields(text) {
		fields = append(fields, field{Field: f, Text: text[f.Start:f.End]})
		b.WriteString(strings.TrimRight(text[at:f.Start], " \t"))
		at = f.End
	}
	b.WriteString(text[at:])
	return strings.TrimSpace(b.String()), fields, id
}
