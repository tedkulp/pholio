package app_test

import (
	"context"
	"testing"

	"github.com/tedkulp/pholio/internal/app"
	"github.com/tedkulp/pholio/internal/config"
	"github.com/tedkulp/pholio/internal/index"
	"github.com/tedkulp/pholio/internal/seam/seamtest"
)

func taskPaths(ix *index.Index) map[string]bool {
	got := map[string]bool{}
	for _, t := range ix.Tasks() {
		got[t.Path] = true
	}
	return got
}

func TestTemplateFoldersFollowDailyTemplateAcrossF8(t *testing.T) {
	fsys := seamtest.NewMemFS(map[string]string{
		"/vault/.pholio/config.toml": `daily_template = "tpl/daily.md"` + "\n",
		"/vault/a.md":                "- [ ] real\n",
		"/vault/tpl/daily.md":        "- [ ] from tpl\n",
		"/vault/templates/x.md":      "- [ ] from templates\n",
	})
	ix := index.New(fsys, "/vault")
	if err := ix.Scan(context.Background()); err != nil {
		t.Fatal(err)
	}
	s, err := config.Startup(fsys, dirs, "/home/u", "/vault/a.md")
	if err != nil {
		t.Fatal(err)
	}
	m, err := app.New(app.Deps{FS: fsys, Index: ix}, "/vault/a.md")
	if err != nil {
		t.Fatal(err)
	}
	m = resize(m.WithSession(s), 60, 6)

	if got := taskPaths(ix); !got["a.md"] || got["tpl/daily.md"] || got["templates/x.md"] {
		t.Errorf("Task Notes at start = %v, want only a.md", got)
	}

	if err := fsys.WriteFile("/vault/.pholio/config.toml", []byte(`daily_template = "other/daily.md"`+"\n")); err != nil {
		t.Fatal(err)
	}
	_ = press(m, f8)

	if got := taskPaths(ix); !got["a.md"] || !got["tpl/daily.md"] || got["templates/x.md"] {
		t.Errorf("Task Notes after F8 = %v, want a.md and tpl/daily.md", got)
	}
}
