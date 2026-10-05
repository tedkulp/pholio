package app

import (
	"slices"
	"testing"
)

func TestMatchIsLiteralAndSmartCase(t *testing.T) {
	tests := []struct {
		query, line string
		want        [][2]int
	}{
		{"alpha", "Alpha and ALPHA and alpha", [][2]int{{0, 5}, {10, 15}, {20, 25}}},
		{"Alpha", "Alpha and ALPHA and alpha", [][2]int{{0, 5}}},
		{"a.b", "axb a.b", [][2]int{{4, 7}}},
		{"[[", "see [[x]]", [][2]int{{4, 6}}},
		{"aa", "aaaa", [][2]int{{0, 2}, {2, 4}}},
		{"x", "ÄÖ x", [][2]int{{5, 6}}},
		{"öl", "Öl und öl", [][2]int{{0, 3}, {8, 11}}},
		{"Öl", "Öl und öl", [][2]int{{0, 3}}},
		{"", "anything", nil},
		{"zz", "nope", nil},
	}
	for _, tt := range tests {
		if got := match(tt.query, tt.line); !slices.Equal(got, tt.want) {
			t.Errorf("match(%q, %q) = %v, want %v", tt.query, tt.line, got, tt.want)
		}
	}
}

func TestSnippetTrimsIndentAndLongLeadIns(t *testing.T) {
	h := grepHit{text: "\t  - find me", marks: [][2]int{{5, 9}}}
	text, marks := h.snippet()
	if text != "- find me" || marks[0] != [2]int{2, 6} {
		t.Errorf("snippet = %q %v, want %q [2 6]", text, marks, "- find me")
	}

	long := "0123456789012345678901234567890123456789 needle"
	h = grepHit{text: long, marks: [][2]int{{41, 47}}}
	text, marks = h.snippet()
	if got := text[marks[0][0]:marks[0][1]]; got != "needle" || text[:len("…")] != "…" {
		t.Errorf("snippet = %q, mark covers %q", text, got)
	}
}
