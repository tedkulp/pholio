package fuzzy_test

import (
	"slices"
	"testing"

	"github.com/tedkulp/pholio/internal/fuzzy"
)

func TestScoreMatchesASubsequenceIgnoringCase(t *testing.T) {
	for _, tc := range []struct {
		query, text string
		ok          bool
	}{
		{"", "anything", true},
		{"mtg", "Meeting notes", true},
		{"MTG", "meeting", true},
		{"gtm", "meeting", false},
		{"über", "Über alles", true},
		{"abc", "ab", false},
	} {
		if _, ok := fuzzy.Score(tc.query, tc.text); ok != tc.ok {
			t.Errorf("Score(%q, %q) ok = %v, want %v", tc.query, tc.text, ok, tc.ok)
		}
	}
}

func TestBetterMatchesScoreHigher(t *testing.T) {
	for _, tc := range []struct {
		query, better, worse string
	}{
		{"beta", "beta", "the abecedarian tale"}, // contiguous beats scattered
		{"pn", "project-notes", "happens"},       // word starts beat mid-word
		{"ali", "alice", "malice"},               // a prefix beats an inner run
		{"note", "note", "notebook-archive"},     // shorter beats longer
	} {
		b, okB := fuzzy.Score(tc.query, tc.better)
		w, okW := fuzzy.Score(tc.query, tc.worse)
		if !okB || !okW || b <= w {
			t.Errorf("%q: %q scored %d (%v), %q scored %d (%v); want the first higher",
				tc.query, tc.better, b, okB, tc.worse, w, okW)
		}
	}
}

func TestRankOrdersByBestFieldThenByIndex(t *testing.T) {
	cands := [][]string{
		{"malice", ""},          // 0: inner match
		{"zettel", "Alice"},     // 1: matches by its second field
		{"bob", "nobody"},       // 2: no match
		{"alice", "Alice"},      // 3: prefix
		{"also-alice", "Alice"}, // 4: same best score as 1 (title), later index
	}
	got := fuzzy.Rank("alice", len(cands), func(i int) []string { return cands[i] })
	var idx []int
	for _, m := range got {
		idx = append(idx, m.Index)
	}
	// 1, 3 and 4 each have a field that is exactly "alice"; ties keep
	// the given order. The inner match comes last and "bob" not at all.
	if want := []int{1, 3, 4, 0}; !slices.Equal(idx, want) {
		t.Errorf("Rank order = %v, want %v", idx, want)
	}
}
