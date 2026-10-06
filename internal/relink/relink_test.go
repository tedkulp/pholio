package relink_test

import (
	"maps"
	"slices"
	"testing"

	"github.com/tedkulp/pholio/internal/index"
	"github.com/tedkulp/pholio/internal/relink"
)

// vault parses Notes given as path → contents.
func vault(files map[string]string) []index.Note {
	var notes []index.Note
	for _, p := range slices.Sorted(maps.Keys(files)) {
		notes = append(notes, index.Parse(p, []byte(files[p])))
	}
	return notes
}

// base is a small Vault like the links fixture.
var base = map[string]string{
	"index.md":        "Start at [[alpha]] or [[alpha#Intro|the intro]].\nThen [[beta]], [[dup]], [[c/dup]] and [[missing]].\nRelative: [gamma](sub/gamma.md).\n",
	"alpha.md":        "# Alpha\n\n## Intro\n\nAlpha points on to [[beta]] and [[#Intro]].\n",
	"a/dup.md":        "# Dup in a\n",
	"b/c/dup.md":      "# Dup in b/c\n",
	"sub/gamma.md":    "Back up to [alpha](../alpha.md#Intro).\n",
	"deep/er/beta.md": "Beta links to [[er/beta]] itself.\n",
	"alpha.sync-conflict-20261001-101010-ABCDEFG.md": "Conflict copy: [[beta]].\n",
}

func with(over map[string]string) map[string]string {
	m := maps.Clone(base)
	maps.Copy(m, over)
	return m
}

func TestPlanRewritesLinksAfterAMove(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		moves map[string]string
		want  map[string]string // the old path of each edited Note → its new contents
	}{
		{
			name:  "renaming a Note rewrites each WikiLink form, keeping #heading and |alias",
			files: base,
			moves: map[string]string{"alpha.md": "omega.md"},
			want: map[string]string{
				"index.md":     "Start at [[omega]] or [[omega#Intro|the intro]].\nThen [[beta]], [[dup]], [[c/dup]] and [[missing]].\nRelative: [gamma](sub/gamma.md).\n",
				"sub/gamma.md": "Back up to [alpha](../omega.md#Intro).\n",
			},
		},
		{
			name:  "moving a Note whose name stays unique changes no bare WikiLink",
			files: base,
			moves: map[string]string{"deep/er/beta.md": "elsewhere/beta.md"},
			want: map[string]string{
				"deep/er/beta.md": "Beta links to [[beta]] itself.\n",
			},
		},
		{
			name:  "a name that is already taken gets the shortest folder prefix that is unique",
			files: base,
			moves: map[string]string{"deep/er/beta.md": "x/dup.md"},
			want: map[string]string{
				"index.md":        "Start at [[alpha]] or [[alpha#Intro|the intro]].\nThen [[x/dup]], [[dup]], [[c/dup]] and [[missing]].\nRelative: [gamma](sub/gamma.md).\n",
				"alpha.md":        "# Alpha\n\n## Intro\n\nAlpha points on to [[x/dup]] and [[#Intro]].\n",
				"deep/er/beta.md": "Beta links to [[x/dup]] itself.\n",
			},
		},
		{
			name:  "a folder-qualified Link that no longer resolves is rewritten",
			files: base,
			moves: map[string]string{"b/c/dup.md": "b/d/dup.md"},
			want: map[string]string{
				"index.md": "Start at [[alpha]] or [[alpha#Intro|the intro]].\nThen [[beta]], [[dup]], [[d/dup]] and [[missing]].\nRelative: [gamma](sub/gamma.md).\n",
			},
		},
		{
			name:  "a new Note that takes over a name rewrites Links to the old holder",
			files: base,
			moves: map[string]string{"index.md": "beta.md"},
			want: map[string]string{
				"index.md": "Start at [[alpha]] or [[alpha#Intro|the intro]].\nThen [[er/beta]], [[dup]], [[c/dup]] and [[missing]].\nRelative: [gamma](sub/gamma.md).\n",
				"alpha.md": "# Alpha\n\n## Intro\n\nAlpha points on to [[er/beta]] and [[#Intro]].\n",
			},
		},
		{
			name:  "a moved Note's own relative Markdown Links are rewritten",
			files: base,
			moves: map[string]string{"sub/gamma.md": "gamma.md"},
			want: map[string]string{
				"index.md":     "Start at [[alpha]] or [[alpha#Intro|the intro]].\nThen [[beta]], [[dup]], [[c/dup]] and [[missing]].\nRelative: [gamma](gamma.md).\n",
				"sub/gamma.md": "Back up to [alpha](alpha.md#Intro).\n",
			},
		},
		{
			name:  "moving a folder moves every Note in it",
			files: base,
			moves: map[string]string{"sub/gamma.md": "new folder/sub/gamma.md"},
			want: map[string]string{
				"index.md":     "Start at [[alpha]] or [[alpha#Intro|the intro]].\nThen [[beta]], [[dup]], [[c/dup]] and [[missing]].\nRelative: [gamma](new%20folder/sub/gamma.md).\n",
				"sub/gamma.md": "Back up to [alpha](../../alpha.md#Intro).\n",
			},
		},
		{
			name: "Links in code, dangling Links and Conflict Notes are left alone",
			files: with(map[string]string{
				"code.md": "```\n[[alpha]]\n```\nInline `[[alpha]]` and [[nowhere]].\n",
			}),
			moves: map[string]string{"alpha.md": "omega.md"},
			want: map[string]string{
				"index.md":     "Start at [[omega]] or [[omega#Intro|the intro]].\nThen [[beta]], [[dup]], [[c/dup]] and [[missing]].\nRelative: [gamma](sub/gamma.md).\n",
				"sub/gamma.md": "Back up to [alpha](../omega.md#Intro).\n",
			},
		},
		{
			name:  "the case a Link was written in is kept while it still resolves",
			files: with(map[string]string{"case.md": "[[BETA]] and [[Alpha]]\n"}),
			moves: map[string]string{"alpha.md": "Omega.md"},
			want: map[string]string{
				"index.md":     "Start at [[Omega]] or [[Omega#Intro|the intro]].\nThen [[beta]], [[dup]], [[c/dup]] and [[missing]].\nRelative: [gamma](sub/gamma.md).\n",
				"sub/gamma.md": "Back up to [alpha](../Omega.md#Intro).\n",
				"case.md":      "[[BETA]] and [[Omega]]\n",
			},
		},
		{
			name:  "nothing to rewrite gives no edits",
			files: base,
			moves: map[string]string{"a/dup.md": "a/dup.md"},
			want:  map[string]string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := map[string]string{}
			for _, e := range relink.Plan(vault(tt.files), tt.moves) {
				got[e.Path] = e.Contents
			}
			for p, want := range tt.want {
				if got[p] != want {
					t.Errorf("%s =\n%q\nwant\n%q", p, got[p], want)
				}
			}
			for p := range got {
				if _, ok := tt.want[p]; !ok {
					t.Errorf("unexpected edit of %s:\n%q", p, got[p])
				}
			}
		})
	}
}

func TestPlanCountsChangedLinksAndNamesTheNotesNewPaths(t *testing.T) {
	edits := relink.Plan(vault(base), map[string]string{"alpha.md": "omega.md", "sub/gamma.md": "g/gamma.md"})
	got := map[string]int{}
	to := map[string]string{}
	for _, e := range edits {
		got[e.Path] = e.Links
		to[e.Path] = e.To
	}
	want := map[string]int{"index.md": 3, "sub/gamma.md": 1}
	if !maps.Equal(got, want) {
		t.Errorf("Links per Note = %v, want %v", got, want)
	}
	if to["sub/gamma.md"] != "g/gamma.md" || to["index.md"] != "index.md" {
		t.Errorf("To = %v, want sub/gamma.md → g/gamma.md and index.md unmoved", to)
	}
}
