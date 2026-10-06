package index_test

import (
	"reflect"
	"testing"

	"github.com/tedkulp/pholio/internal/index"
)

func TestResolveFindsNotesByNameAnywhereInTheVault(t *testing.T) {
	ix, _ := scanned(t, "links")
	tests := []struct {
		from string
		link index.Link
		want index.Resolution
	}{
		{"index.md", wiki("alpha"), index.Resolution{Path: "alpha.md"}},
		{"index.md", wiki("ALPHA"), index.Resolution{Path: "alpha.md"}},
		{"index.md", wiki("alpha.md"), index.Resolution{Path: "alpha.md"}},
		{"index.md", wiki("beta"), index.Resolution{Path: "deep/er/beta.md"}},
		{"index.md", wiki("er/beta"), index.Resolution{Path: "deep/er/beta.md"}},
		{"index.md", wiki("c/dup"), index.Resolution{Path: "b/c/dup.md"}},
		{"index.md", wiki("b/c/dup"), index.Resolution{Path: "b/c/dup.md"}},
		{"index.md", wiki("/a/dup"), index.Resolution{Path: "a/dup.md"}},
		{"alpha.md", index.Link{Kind: index.WikiLink, Heading: "Intro"}, index.Resolution{Path: "alpha.md"}},
		{"index.md", md("sub/gamma.md"), index.Resolution{Path: "sub/gamma.md"}},
		{"sub/gamma.md", md("../alpha.md"), index.Resolution{Path: "alpha.md"}},
	}
	for _, tt := range tests {
		got, ok := ix.Resolve(tt.from, tt.link)
		if !ok || !reflect.DeepEqual(got, tt.want) {
			t.Errorf("Resolve(%q, %+v) = %+v, %v; want %+v", tt.from, tt.link, got, ok, tt.want)
		}
	}
}

func TestResolvePicksTheShortestPathAndFlagsAmbiguity(t *testing.T) {
	ix, _ := scanned(t, "links")
	got, ok := ix.Resolve("index.md", wiki("dup"))
	want := index.Resolution{Path: "a/dup.md", Ambiguous: true, Candidates: []string{"a/dup.md", "b/c/dup.md"}}
	if !ok || !reflect.DeepEqual(got, want) {
		t.Errorf("Resolve(dup) = %+v, %v; want %+v", got, ok, want)
	}
}

func TestResolveLeavesDanglingAndConflictTargetsUnresolved(t *testing.T) {
	ix, _ := scanned(t, "links")
	for _, l := range []index.Link{
		wiki("missing"),
		wiki("x/alpha"),
		wiki("alpha.sync-conflict-20261001-101010-ABCDEFG"),
		md("alpha.sync-conflict-20261001-101010-ABCDEFG.md"),
		md("nowhere.md"),
		md("../../outside.md"),
	} {
		if got, ok := ix.Resolve("index.md", l); ok {
			t.Errorf("Resolve(%+v) = %+v, want unresolved", l, got)
		}
	}
}

func TestBacklinksListLinkingNotesWithContext(t *testing.T) {
	ix, _ := scanned(t, "links")
	var got []string
	for _, b := range ix.Backlinks("alpha.md") {
		got = append(got, b.Source+": "+b.Context)
	}
	want := []string{
		"index.md: Start at [[alpha]] or [[alpha#Intro|the intro]].",
		"index.md: Start at [[alpha]] or [[alpha#Intro|the intro]].",
		"sub/gamma.md: Back up to [alpha](../alpha.md).",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Backlinks(alpha.md)\n got %q\nwant %q", got, want)
	}

	// Conflict copies and dot-directories don't count as sources.
	got = nil
	for _, b := range ix.Backlinks("deep/er/beta.md") {
		got = append(got, b.Source)
	}
	if want := []string{"alpha.md", "index.md"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Backlinks(beta) = %v, want %v", got, want)
	}

	// An ambiguous name links to the shortest path only.
	if bl := ix.Backlinks("b/c/dup.md"); len(bl) != 1 || bl[0].Link.Target != "c/dup" {
		t.Errorf("Backlinks(b/c/dup.md) = %+v", bl)
	}
}

func wiki(target string) index.Link { return index.Link{Kind: index.WikiLink, Target: target} }

func md(target string) index.Link { return index.Link{Kind: index.MarkdownLink, Target: target} }

func TestADateLinkReachesANestedDailyNote(t *testing.T) {
	ix, root := scanned(t, "links")
	ix.Update(root+"/daily/2026/10/2026-10-06.md", []byte("# Tuesday\n"))

	if got, ok := ix.Resolve("index.md", wiki("2026-10-06")); !ok || got.Path != "daily/2026/10/2026-10-06.md" {
		t.Errorf("Resolve(2026-10-06) = %+v, %v", got, ok)
	}
}
