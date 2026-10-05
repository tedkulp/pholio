package engine

import (
	"regexp"
	"strings"
)

// searchState is the last "/" or "?" pattern.
type searchState struct {
	re    *regexp.Regexp
	input string // the pattern as typed, for messages
	back  bool   // the last search was "?"
	hl    bool   // highlight matches; :noh clears it until the next search
}

// SetSearch makes query the last search pattern, as if typed after "/".
// The query is literal text and is matched with smartcase. Vault search uses
// it so that n and N keep working in the opened Note.
func (e *Engine) SetSearch(query string) {
	e.setSearch(regexp.QuoteMeta(query), query)
	e.search.back = false
}

// Matches returns the [start, end) byte ranges that match the search pattern
// on line i, or nil when there is nothing to highlight.
func (e *Engine) Matches(i int) [][]int {
	if e.search.re == nil || !e.search.hl {
		return nil
	}
	return e.search.re.FindAllStringIndex(e.line(i), -1)
}

// compileSearch sets the pattern typed after "/" or "?". An empty pattern
// reuses the last one, and an invalid regexp is matched literally.
func (e *Engine) compileSearch(pat string) {
	if pat == "" {
		e.search.hl = e.search.re != nil
		return
	}
	if _, err := regexp.Compile(pat); err != nil {
		e.setSearch(regexp.QuoteMeta(pat), pat)
		return
	}
	e.setSearch(pat, pat)
}

func (e *Engine) setSearch(re, input string) {
	if strings.ToLower(input) == input {
		re = "(?i)" + re // smartcase
	}
	e.search.re = regexp.MustCompile(re)
	e.search.input, e.search.hl = input, true
}

// searchFrom returns the start of the nth match after p (or before it when
// back is set), wrapping around the buffer.
func (e *Engine) searchFrom(p Pos, back bool, n int) (Pos, bool) {
	if e.search.re == nil {
		e.Msg = "E35: No previous regular expression"
		return p, false
	}
	e.search.hl = true
	ms := e.search.re.FindAllStringIndex(e.Buf.String(), -1)
	if len(ms) == 0 {
		e.Msg = "E486: Pattern not found: " + e.search.input
		return p, false
	}
	o := e.Buf.offset(p)
	for range max(n, 1) {
		idx := -1
		if back {
			for i := len(ms) - 1; i >= 0; i-- {
				if ms[i][0] < o {
					idx = i
					break
				}
			}
			if idx < 0 {
				idx = len(ms) - 1
				e.Msg = "search hit TOP, continuing at BOTTOM"
			}
		} else {
			for i, m := range ms {
				if m[0] > o {
					idx = i
					break
				}
			}
			if idx < 0 {
				idx = 0
				e.Msg = "search hit BOTTOM, continuing at TOP"
			}
		}
		o = ms[idx][0]
	}
	return e.Buf.posAt(o), true
}

// searchMotion is n (reverse false) or N (reverse true).
func searchMotion(reverse bool) motionFn {
	return func(e *Engine, n int, _ string) (Pos, kind, bool) {
		p, ok := e.searchFrom(e.Cur, e.search.back != reverse, n)
		return p, excl, ok
	}
}
