package index

import (
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/tedkulp/pholio/internal/dates"
)

// Format is a Task Format: the syntax Task Metadata is written in.
type Format string

// The Task Formats, those of Obsidian's Tasks plugin.
const (
	Dataview Format = "dataview" // [due:: 2026-10-10]
	Emoji    Format = "emoji"    // 📅 2026-10-10
)

// Field is one Task Metadata field found in a line.
type Field struct {
	// Key is the field's Dataview name: "due", "completion", "priority",
	// "start", "scheduled", "created" or "cancelled".
	Key string
	// Value is the value as written, trimmed. A priority emoji's Value is
	// its level name.
	Value  string
	Format Format
	// Start and End are the byte span of the whole field; ValStart and
	// ValEnd that of the value, empty for a priority emoji.
	Start, End, ValStart, ValEnd int
}

// priorities are the priority levels, highest first.
var priorities = []string{"highest", "high", "medium", "low", "lowest"}

// dataviewKeys are the Dataview field names that are Task Metadata.
var dataviewKeys = []string{"due", "completion", "priority", "start", "scheduled", "created", "cancelled"}

// emojiDates maps each date emoji to its Dataview name.
var emojiDates = map[rune]string{
	'📅': "due", '📆': "due", '🗓': "due",
	'✅': "completion",
	'⏳': "scheduled", '⌛': "scheduled",
	'🛫': "start",
	'➕': "created",
	'❌': "cancelled",
}

// emojiPriorities maps each priority emoji to its level.
var emojiPriorities = map[rune]string{
	'🔺': "highest", '⏫': "high", '🔼': "medium", '🔽': "low", '⏬': "lowest",
}

// dataviewFields match "[key:: value]" and "(key:: value)".
var dataviewFields = []*regexp.Regexp{
	regexp.MustCompile(`\[([a-z]+)::[ \t]*([^\]]*?)[ \t]*\]`),
	regexp.MustCompile(`\(([a-z]+)::[ \t]*([^)]*?)[ \t]*\)`),
}

// Fields finds the Task Metadata fields in s, in either Task Format, in the
// order they appear. Fields don't overlap.
func Fields(s string) []Field {
	var fs []Field
	for _, re := range dataviewFields {
		for _, m := range re.FindAllStringSubmatchIndex(s, -1) {
			key := s[m[2]:m[3]]
			if !slices.Contains(dataviewKeys, key) {
				continue
			}
			v := s[m[4]:m[5]]
			if key == "priority" {
				v = strings.ToLower(v)
			}
			fs = append(fs, Field{Key: key, Value: v, Format: Dataview, Start: m[0], End: m[1], ValStart: m[4], ValEnd: m[5]})
		}
	}
	fs = append(fs, emojiFields(s)...)
	slices.SortFunc(fs, func(a, b Field) int { return a.Start - b.Start })
	kept := fs[:0]
	for _, f := range fs {
		if len(kept) == 0 || f.Start >= kept[len(kept)-1].End {
			kept = append(kept, f)
		}
	}
	return kept
}

// isEmojiDate reports whether v can follow the date emoji for key: an ISO
// date, or for a due or done date a relative one that dates.Parse reads.
func isEmojiDate(key, v string) bool {
	if isoDate.MatchString(v) {
		return true
	}
	_, ok := dates.Parse(v, time.Time{})
	return ok && TakesRelativeDate(key)
}

// TakesRelativeDate reports whether the field key may hold a relative date
// ("tomorrow", "+3d") to be expanded: the due and done dates.
func TakesRelativeDate(key string) bool { return key == "due" || key == "completion" }

// isoDate matches a YYYY-MM-DD date.
var isoDate = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// emojiFields finds the emoji fields in s: a priority emoji alone, or a
// date emoji, then blanks, then a date (see isEmojiDate).
func emojiFields(s string) []Field {
	var fs []Field
	for i := 0; i < len(s); {
		r, n := utf8.DecodeRuneInString(s[i:])
		start, end := i, i+n
		i = end
		pri, isPri := emojiPriorities[r]
		key, isDate := emojiDates[r]
		if !isPri && !isDate {
			continue
		}
		if strings.HasPrefix(s[end:], "\uFE0F") {
			end += len("\uFE0F")
		}
		if isPri {
			fs = append(fs, Field{Key: "priority", Value: pri, Format: Emoji, Start: start, End: end, ValStart: end, ValEnd: end})
			i = end
			continue
		}
		vs := end + len(s[end:]) - len(strings.TrimLeft(s[end:], " \t"))
		ve := vs
		for ve < len(s) && isValueByte(s[ve]) {
			ve++
		}
		if !isEmojiDate(key, s[vs:ve]) {
			continue
		}
		fs = append(fs, Field{Key: key, Value: s[vs:ve], Format: Emoji, Start: start, End: ve, ValStart: vs, ValEnd: ve})
		i = ve
	}
	return fs
}

// isValueByte reports whether b can be part of an emoji field's value: an
// ISO date, or a relative one such as "fri" or "+3d".
func isValueByte(b byte) bool {
	return b >= '0' && b <= '9' || b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b == '-' || b == '+'
}
