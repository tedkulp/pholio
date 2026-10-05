package index_test

import (
	"reflect"
	"testing"

	"github.com/tedkulp/pholio/internal/index"
)

func TestParseFindsLinksWithPositions(t *testing.T) {
	src := "see [[plain]] and [[dir/name#Some Heading|the alias]].\n" +
		"[[#Local]] then ![[embed]] and [text](sub/other.md#Top), [x](../up%20one.md)\n" +
		"`[[in code]]` [web](https://example.com) [img](pic.png) [[ ]] [[unclosed\n" +
		"```\n[[fenced]]\n```\n" +
		"- [ ] task with [[task link]]\n"
	n := index.Parse("notes/a.md", []byte(src))
	want := []index.Link{
		{Kind: index.WikiLink, Target: "plain", Line: 0, Start: 4, End: 13},
		{Kind: index.WikiLink, Target: "dir/name", Heading: "Some Heading", Alias: "the alias", Line: 0, Start: 18, End: 53},
		{Kind: index.WikiLink, Heading: "Local", Line: 1, Start: 0, End: 10},
		{Kind: index.WikiLink, Target: "embed", Line: 1, Start: 17, End: 26},
		{Kind: index.MarkdownLink, Target: "sub/other.md", Heading: "Top", Alias: "text", Line: 1, Start: 31, End: 55},
		{Kind: index.MarkdownLink, Target: "../up one.md", Alias: "x", Line: 1, Start: 57, End: 76},
		{Kind: index.WikiLink, Target: "task link", Line: 6, Start: 16, End: 29},
	}
	if !reflect.DeepEqual(n.Links, want) {
		t.Errorf("Links\n got %+v\nwant %+v", n.Links, want)
	}
	for _, l := range n.Links {
		line := lineOf(src, l.Line)
		if l.End > len(line) {
			t.Errorf("link %+v past end of line", l)
		}
	}
}

func lineOf(s string, n int) string {
	for range n {
		for i := 0; i < len(s); i++ {
			if s[i] == '\n' {
				s = s[i+1:]
				break
			}
		}
	}
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			return s[:i]
		}
	}
	return s
}
