package engine

import "testing"

func TestWordMotions(t *testing.T) {
	runTable(t, []tcase{
		{"w", "|foo bar.baz", "w", "foo |bar.baz"},
		{"3w", "|foo bar.baz", "3w", "foo bar.|baz"},
		{"w across lines", "|foo\n\nbar", "w", "foo\n|\nbar"},
		{"w from empty line", "foo\n|\nbar", "w", "foo\n\n|bar"},
		{"W", "|foo bar.baz qux", "2W", "foo bar.baz |qux"},
		{"b", "foo bar|", "b", "foo |bar"},
		{"b over punct", "foo.ba|r", "2b", "foo|.bar"},
		{"b across lines", "foo\n|bar", "b", "|foo\nbar"},
		{"B", "foo.bar ba|z", "2B", "|foo.bar baz"},
		{"e", "|foo bar", "e", "fo|o bar"},
		{"2e", "|foo bar", "2e", "foo ba|r"},
		{"E", "|a.b c", "E", "a.|b c"},
		{"ge", "foo bar|", "ge", "fo|o bar"},
		{"gE", "a.b c|d", "gE", "a.|b cd"},
		{"w over CJK word", "|日本 語", "w", "日本 |語"},
	})
}

func TestFindMotions(t *testing.T) {
	runTable(t, []tcase{
		{"f", "|a,b,c", "f,", "a|,b,c"},
		{"2f", "|a,b,c", "2f,", "a,b|,c"},
		{"f missing stays", "|abc", "fz", "|abc"},
		{"F", "a,b,|c", "F,", "a,b|,c"},
		{"t", "|a,b,c", "t,", "|a,b,c"},
		{"t from before", "|ab,c", "t,", "a|b,c"},
		{"T", "a,b|c", "T,", "a,|bc"},
		{"t ;", "|a,b,c", "t,;", "a,|b,c"},
		{"f ;", "|a,b,c", "f,;", "a,b|,c"},
		{"f ,", "|a,b,c", "2f,F,,", "a,b|,c"},
		{"F ,", "|a,b,c,d", "3f,F,,", "a,b,c|,d"},
		{"f matches grapheme base", "|a 👍🏽 b", "f👍", "a |👍🏽 b"},
		{"cjk f", "|日本語", "f語", "日本|語"},
	})
}

func TestBlockMotions(t *testing.T) {
	runTable(t, []tcase{
		{"}", "|a\nb\n\nc", "}", "a\nb\n|\nc"},
		{"2}", "|a\n\nb\n\nc", "2}", "a\n\nb\n|\nc"},
		{"} at end", "a\n|b", "}", "a\n|b"},
		{"{", "a\n\nb\n|c", "{", "a\n|\nb\nc"},
		{"{ at top", "a\n|b", "{", "|a\nb"},
		{"%", "|(a [b] c)", "%", "(a [b] c|)"},
		{"% back", "(a [b] c|)", "%", "|(a [b] c)"},
		{"% finds next bracket", "|x [b] c", "%", "x [b|] c"},
		{"% across lines", "|{\n a\n}", "%", "{\n a\n|}"},
		{"% nested", "|((a))", "%", "((a)|)"},
	})
}
