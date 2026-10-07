package editor_test

import (
	"strings"
	"testing"
)

// bigNote is a 20k-line Note of mixed short and wrapping lines.
func bigNote() string {
	para := "A paragraph line that is long enough to wrap at least once in an eighty column pane, with [[links]] and **bold**.\n"
	var sb strings.Builder
	for i := 0; i < 5000; i++ {
		sb.WriteString("## Heading\n- [ ] a task [due:: 2026-10-10]\n")
		sb.WriteString(para)
		sb.WriteString("\n")
	}
	return sb.String()
}

// BenchmarkFrame20k is one keypress and redraw on a 20k-line Note, jumping
// between the ends of the file so every frame scrolls the whole buffer.
func BenchmarkFrame20k(b *testing.B) {
	m := open(bigNote(), 80, 40)
	keys := []string{"G", "g", "g"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, k := range keys {
			m = m.Update(key(k))
		}
		_, _ = m.View(th)
		_ = m.StatusLine(th, 80)
	}
}

// BenchmarkScrollLine20k is j in the middle of a 20k-line Note, with redraw.
func BenchmarkScrollLine20k(b *testing.B) {
	m := feed(open(bigNote(), 80, 40), "10000gg")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		k := "j"
		if i%2 == 1 {
			k = "k"
		}
		m = m.Update(key(k))
		_, _ = m.View(th)
		_ = m.StatusLine(th, 80)
	}
}
