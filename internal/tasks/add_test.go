package tasks_test

import (
	"strings"
	"testing"

	"github.com/tedkulp/pholio/internal/seam/seamtest"
	"github.com/tedkulp/pholio/internal/tasks"
)

func TestAddAtGoesAfterTheLastTaskUnderTheHeading(t *testing.T) {
	for name, tc := range map[string]struct {
		text string
		want int
	}{
		"after the last Task, skipping fenced code": {
			"# Day\n\n## Tasks\n\n- [ ] a\n  - [x] b\nnote\n\n```\n- [ ] code\n```\n", 6,
		},
		"the section ends at the next heading of the same level": {
			"## Tasks\n- [ ] a\n### Sub\n- [ ] sub\n## Notes\n- [ ] not here\n", 4,
		},
		"right under a heading with no Tasks": {
			"# Day\n## Tasks\n\nsome text\n", 2,
		},
		"end of the file when the heading is missing": {
			"# Day\n- [ ] a\n", 2,
		},
		"a heading in fenced code does not count": {
			"```\n## Tasks\n```\n- [ ] a\n", 4,
		},
	} {
		lines := strings.Split(strings.TrimSuffix(tc.text, "\n"), "\n")
		if got := tasks.AddAt(lines, "## Tasks"); got != tc.want {
			t.Errorf("%s: AddAt = %d, want %d", name, got, tc.want)
		}
	}
}

func TestAddFileInsertsTheLineKeepingLineEndings(t *testing.T) {
	fsys := seamtest.NewMemFS(map[string]string{
		"/v/a.md": "# A\r\n## Tasks\r\n- [ ] a\r\n\r\ntext\r\n",
		"/v/b.md": "# B",
		"/v/c.md": "",
	})

	for _, p := range []string{"/v/a.md", "/v/b.md", "/v/c.md"} {
		if err := tasks.AddFile(fsys, p, "## Tasks", "- [ ] new"); err != nil {
			t.Fatal(err)
		}
	}

	for p, want := range map[string]string{
		"/v/a.md": "# A\r\n## Tasks\r\n- [ ] a\r\n- [ ] new\r\n\r\ntext\r\n",
		"/v/b.md": "# B\n- [ ] new\n",
		"/v/c.md": "- [ ] new\n",
	} {
		if got := read(t, fsys, p); got != want {
			t.Errorf("%s = %q, want %q", p, got, want)
		}
	}
}
