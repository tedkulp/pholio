// Package fuzzy is the fuzzy matcher shared by find Note, [[ autocomplete
// and the other pickers: the query's characters must appear in the text in
// order, ignoring case, and matches score higher when they are contiguous,
// start words and fall in shorter texts.
package fuzzy

import (
	"slices"
	"unicode"
)

// Scoring weights.
const (
	matchScore  = 16 // each matched character
	boundary    = 8  // a match that starts a word
	consecutive = 8  // a match right after the previous one
	gapStart    = 3  // skipping characters between two matches
	gapExtend   = 1  // each further skipped character
	maxLenCost  = 8  // the most a long text loses for its length
)

const none = -1 << 30

// Score reports how well query matches text. ok is false when query's
// characters do not all appear in text, in order. An empty query matches
// everything with score 0.
func Score(query, text string) (score int, ok bool) {
	q := lower(query)
	if len(q) == 0 {
		return 0, true
	}
	orig := []rune(text)
	t := lower(text)
	if len(q) > len(t) || !subsequence(q, t) {
		return 0, false
	}
	n := len(t)
	prev, cur := make([]int, n), make([]int, n)
	for j := range n { // the first query character
		cur[j] = none
		if t[j] == q[0] {
			cur[j] = matchScore + bonus(orig, j)
		}
	}
	for i := 1; i < len(q); i++ {
		prev, cur = cur, prev
		run := none // best prev[k] for k ≤ j-2, less the gap's extension
		for j := range n {
			cur[j] = none
			if j >= 2 {
				run = max(decay(run), prev[j-2])
			}
			if j < i || t[j] != q[i] {
				continue
			}
			best := none
			if prev[j-1] > none {
				best = prev[j-1] + consecutive
			}
			if run > none {
				best = max(best, run-gapStart)
			}
			if best > none {
				cur[j] = best + matchScore + bonus(orig, j)
			}
		}
	}
	score = slices.Max(cur)
	return score - min(maxLenCost, (n-len(q))/4), true
}

func decay(s int) int {
	if s == none {
		return none
	}
	return s - gapExtend
}

// bonus is the extra score for a match at t[j]: the start of the text, a
// character after a separator, or an upper-case letter after a lower-case
// one.
func bonus(t []rune, j int) int {
	if j == 0 {
		return boundary
	}
	p, c := t[j-1], t[j]
	if !unicode.IsLetter(p) && !unicode.IsDigit(p) && (unicode.IsLetter(c) || unicode.IsDigit(c)) {
		return boundary
	}
	if unicode.IsLower(p) && unicode.IsUpper(c) {
		return boundary
	}
	return 0
}

func lower(s string) []rune {
	r := []rune(s)
	for i, c := range r {
		r[i] = unicode.ToLower(c)
	}
	return r
}

func subsequence(q, t []rune) bool {
	i := 0
	for _, c := range t {
		if i < len(q) && c == q[i] {
			i++
		}
	}
	return i == len(q)
}

// Match is one ranked candidate.
type Match struct {
	Index int // the candidate's index
	Score int // its best field's score
}

// Rank scores n candidates against query and returns those that match,
// best first. fields gives candidate i's texts (a filename and a title,
// say); a candidate scores as its best field. Equal scores keep the
// candidates' order, so sort them by preference first.
func Rank(query string, n int, fields func(i int) []string) []Match {
	var out []Match
	for i := range n {
		best, hit := 0, false
		for _, f := range fields(i) {
			if s, ok := Score(query, f); ok && (!hit || s > best) {
				best, hit = s, true
			}
		}
		if hit {
			out = append(out, Match{Index: i, Score: best})
		}
	}
	slices.SortStableFunc(out, func(a, b Match) int { return b.Score - a.Score })
	return out
}
