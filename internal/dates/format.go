package dates

import (
	"strconv"
	"strings"
	"time"
)

// tokens are the moment.js format tokens Format knows, longest first so
// that "MMMM" wins over "MM".
var tokens = []struct {
	tok string
	fn  func(t time.Time) string
}{
	{"YYYY", func(t time.Time) string { return pad(t.Year(), 4) }},
	{"GGGG", func(t time.Time) string { y, _ := t.ISOWeek(); return pad(y, 4) }},
	{"MMMM", func(t time.Time) string { return t.Month().String() }},
	{"DDDD", func(t time.Time) string { return pad(t.YearDay(), 3) }},
	{"dddd", func(t time.Time) string { return t.Weekday().String() }},
	{"MMM", func(t time.Time) string { return t.Month().String()[:3] }},
	{"DDD", func(t time.Time) string { return strconv.Itoa(t.YearDay()) }},
	{"ddd", func(t time.Time) string { return t.Weekday().String()[:3] }},
	{"YY", func(t time.Time) string { return pad(t.Year()%100, 2) }},
	{"MM", func(t time.Time) string { return pad(int(t.Month()), 2) }},
	{"DD", func(t time.Time) string { return pad(t.Day(), 2) }},
	{"Do", func(t time.Time) string { return ordinal(t.Day()) }},
	{"dd", func(t time.Time) string { return t.Weekday().String()[:2] }},
	{"WW", func(t time.Time) string { _, w := t.ISOWeek(); return pad(w, 2) }},
	{"HH", func(t time.Time) string { return pad(t.Hour(), 2) }},
	{"hh", func(t time.Time) string { return pad(hour12(t), 2) }},
	{"mm", func(t time.Time) string { return pad(t.Minute(), 2) }},
	{"ss", func(t time.Time) string { return pad(t.Second(), 2) }},
	{"Q", func(t time.Time) string { return strconv.Itoa((int(t.Month())-1)/3 + 1) }},
	{"M", func(t time.Time) string { return strconv.Itoa(int(t.Month())) }},
	{"D", func(t time.Time) string { return strconv.Itoa(t.Day()) }},
	{"d", func(t time.Time) string { return strconv.Itoa(int(t.Weekday())) }},
	{"E", func(t time.Time) string { return strconv.Itoa((int(t.Weekday())+6)%7 + 1) }},
	{"W", func(t time.Time) string { _, w := t.ISOWeek(); return strconv.Itoa(w) }},
	{"H", func(t time.Time) string { return strconv.Itoa(t.Hour()) }},
	{"h", func(t time.Time) string { return strconv.Itoa(hour12(t)) }},
	{"m", func(t time.Time) string { return strconv.Itoa(t.Minute()) }},
	{"s", func(t time.Time) string { return strconv.Itoa(t.Second()) }},
	{"A", func(t time.Time) string { return strings.ToUpper(meridiem(t)) }},
	{"a", meridiem},
	{"X", func(t time.Time) string { return strconv.FormatInt(t.Unix(), 10) }},
}

// Format renders t with moment.js-style tokens (YYYY-MM-DD, dddd, Do,
// HH:mm, …), so Obsidian templates work unchanged. Text in [brackets] is
// copied as is; any other character that is not a token is kept.
func Format(t time.Time, format string) string {
	var b strings.Builder
	rest := format
next:
	for rest != "" {
		if rest[0] == '[' {
			if end := strings.IndexByte(rest, ']'); end > 0 {
				b.WriteString(rest[1:end])
				rest = rest[end+1:]
				continue
			}
		}
		for _, tk := range tokens {
			if strings.HasPrefix(rest, tk.tok) {
				b.WriteString(tk.fn(t))
				rest = rest[len(tk.tok):]
				continue next
			}
		}
		b.WriteByte(rest[0])
		rest = rest[1:]
	}
	return b.String()
}

func pad(n, width int) string {
	s := strconv.Itoa(n)
	return strings.Repeat("0", max(0, width-len(s))) + s
}

func hour12(t time.Time) int {
	if h := t.Hour() % 12; h != 0 {
		return h
	}
	return 12
}

func meridiem(t time.Time) string {
	if t.Hour() < 12 {
		return "am"
	}
	return "pm"
}

func ordinal(n int) string {
	suffix := "th"
	if n%100 < 11 || n%100 > 13 {
		switch n % 10 {
		case 1:
			suffix = "st"
		case 2:
			suffix = "nd"
		case 3:
			suffix = "rd"
		}
	}
	return strconv.Itoa(n) + suffix
}
