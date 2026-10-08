package engine

import "testing"

func TestIndentOperators(t *testing.T) {
	runTable(t, []tcase{
		{">>", "f|oo", "<gt><gt>", "\t|foo"},
		{"<<", "\tf|oo", "<lt><lt>", "|foo"},
		{"3>>", "|a\nb\nc\nd", "3<gt><gt>", "\t|a\n\tb\n\tc\nd"},
		{"count past the end", "a\n|b\nc", "5<gt><gt>", "a\n\t|b\n\tc"},
		{">j", "|a\nb\nc", "<gt>j", "\t|a\n\tb\nc"},
		{">ip", "|a\nb\n\nc", "<gt>ip", "\t|a\n\tb\n\nc"},
		{">G", "a\n|b\nc", "<gt>G", "a\n\t|b\n\tc"},
		{"charwise motion is linewise", "a|bc\nd", "<gt>l", "\t|abc\nd"},
		{"cursor to first changed line", "a\n|b", "<gt>k", "\t|a\n\tb"},
		{"<< two spaces", "  f|oo", "<lt><lt>", "|foo"},
		{"<< six spaces", "      f|oo", "<lt><lt>", "  |foo"},
		{"<< one level of tabs", "\t\t|foo", "<lt><lt>", "\t|foo"},
		{"<< tab before spaces", "\t  |foo", "<lt><lt>", "  |foo"},
		{"<< no indent", "f|oo", "<lt><lt>", "|foo"},
		{"<j mixed", "|  a\nb\n\tc", "<lt>2j", "|a\nb\nc"},
		{">> empty line", "|", "<gt><gt>", "|"},
		{">> whitespace-only line", "|  ", "<gt><gt>", " | "},
		{"> skips blank lines", "|a\n\nb", "<gt>2j", "\t|a\n\n\tb"},
		{"undo 3>>", "|a\nb\nc", "3<gt><gt>u", "|a\nb\nc"},
		{"dot >>", "|a", "<gt><gt>.", "\t\t|a"},
		{"dot keeps count", "|a\nb\nc", "2<gt><gt>j.", "\ta\n\t\t|b\n\tc"},
		{"dot motion", "|a\nb\nc", "<gt>j.", "\t\t|a\n\t\tb\nc"},
		{"V>", "|a\nb\nc", "Vj<gt>", "\t|a\n\tb\nc"},
		{"v>", "a|b\ncd\ne", "vj<gt>", "\t|ab\n\tcd\ne"},
		{"V<", "\ta\n\t|b", "Vk<lt>", "|a\nb"},
		{"visual > back to normal", "|a\nb", "Vj<gt>x", "|\t\n\tb"},
		{"undo visual >", "|a\nb", "Vj<gt>u", "|a\nb"},
	})
}
