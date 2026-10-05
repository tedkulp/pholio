package fuzzy_test

import (
	"fmt"
	"testing"

	"github.com/tedkulp/pholio/internal/fuzzy"
)

// BenchmarkRank50k ranks 50k Notes (filename and title) for one keystroke.
func BenchmarkRank50k(b *testing.B) {
	fields := make([][]string, 50_000)
	for i := range fields {
		fields[i] = []string{
			fmt.Sprintf("202610%02d%04d-some-zettel-about-%d", i%28, i, i),
			fmt.Sprintf("Some Zettel about topic %d and more", i),
		}
	}
	for _, q := range []string{"z", "zettel", "about 123"} {
		b.Run(q, func(b *testing.B) {
			for b.Loop() {
				fuzzy.Rank(q, len(fields), func(i int) []string { return fields[i] })
			}
		})
	}
}
