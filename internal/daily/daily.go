// Package daily finds, creates and fills Daily Notes: one Note per day at
// <daily_folder>/[<daily_subfolder>/]YYYY-MM-DD.md, made from the daily
// Template the first time that day is opened.
package daily

import (
	"regexp"
	"time"

	"github.com/tedkulp/pholio/internal/dates"
)

// Vars are what a Template's placeholders expand to.
type Vars struct {
	Day   time.Time // the Note's own day: {{date}}, {{yesterday}}, {{tomorrow}}
	Now   time.Time // the wall clock: {{time}}
	Title string    // the Note's name without .md: {{title}}
}

var placeholder = regexp.MustCompile(`\{\{\s*(\w+)(?::([^}]*))?\s*\}\}`)

// Render expands the placeholders in a Template: {{date}}, {{date:FMT}},
// {{time}}, {{time:FMT}}, {{title}}, {{yesterday}} and {{tomorrow}}.
// Formats use moment.js tokens. Unknown placeholders are left as they are.
func Render(tmpl string, v Vars) string {
	return placeholder.ReplaceAllStringFunc(tmpl, func(s string) string {
		m := placeholder.FindStringSubmatch(s)
		name, format := m[1], m[2]
		withDefault := func(def string) string {
			if format == "" {
				return def
			}
			return format
		}
		switch name {
		case "date":
			return dates.Format(v.Day, withDefault("YYYY-MM-DD"))
		case "time":
			return dates.Format(v.Now, withDefault("HH:mm"))
		case "title":
			return v.Title
		case "yesterday":
			return dates.Format(v.Day.AddDate(0, 0, -1), withDefault("YYYY-MM-DD"))
		case "tomorrow":
			return dates.Format(v.Day.AddDate(0, 0, 1), withDefault("YYYY-MM-DD"))
		}
		return s
	})
}
