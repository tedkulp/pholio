package daily_test

import (
	"testing"
	"time"

	"github.com/tedkulp/pholio/internal/daily"
)

func TestRenderPlaceholders(t *testing.T) {
	vars := daily.Vars{
		Day:   time.Date(2026, 10, 5, 0, 0, 0, 0, time.Local),
		Now:   time.Date(2026, 10, 9, 21, 4, 0, 0, time.Local), // later than Day
		Title: "2026-10-05",
	}
	for tmpl, want := range map[string]string{
		"{{date}}":                  "2026-10-05",
		"{{date:dddd, MMMM Do}}":    "Monday, October 5th",
		"{{time}}":                  "21:04",
		"{{time:h:mm A}}":           "9:04 PM",
		"{{title}}":                 "2026-10-05",
		"{{yesterday}}":             "2026-10-04",
		"{{tomorrow}}":              "2026-10-06",
		"[[{{yesterday}}]]":         "[[2026-10-04]]",
		"{{ date }}":                "2026-10-05",
		"{{unknown}} {{date":        "{{unknown}} {{date",
		"# {{date}}\n\n## Tasks\n":  "# 2026-10-05\n\n## Tasks\n",
		"{{date:YYYY}}/{{date:MM}}": "2026/10",
	} {
		if got := daily.Render(tmpl, vars); got != want {
			t.Errorf("Render(%q) = %q, want %q", tmpl, got, want)
		}
	}
}
