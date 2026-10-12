package theme_test

import (
	"image/color"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/tedkulp/pholio/internal/theme"
)

func TestParseColor(t *testing.T) {
	tests := []struct {
		in   string
		want color.Color
	}{
		{"#ff8000", color.RGBA{R: 0xff, G: 0x80, B: 0x00, A: 0xff}},
		{"#FF8000", color.RGBA{R: 0xff, G: 0x80, B: 0x00, A: 0xff}},
		{"#f80", color.RGBA{R: 0xff, G: 0x88, B: 0x00, A: 0xff}},
		{"0", ansi.BasicColor(0)},
		{"15", ansi.BasicColor(15)},
		{"16", ansi.IndexedColor(16)},
		{"255", ansi.IndexedColor(255)},
	}
	for _, tt := range tests {
		got, err := theme.ParseColor(tt.in)
		if err != nil {
			t.Errorf("ParseColor(%q): %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseColor(%q) = %#v, want %#v", tt.in, got, tt.want)
		}
	}
}

func TestParseColorRejects(t *testing.T) {
	for _, in := range []string{"", "red", "#ff80", "#gg0000", "256", "-1", "1.5", "#"} {
		if c, err := theme.ParseColor(in); err == nil {
			t.Errorf("ParseColor(%q) = %v, want an error", in, c)
		}
	}
}

// rgb is c as 8-bit red, green and blue.
func rgb(c color.Color) [3]uint32 {
	r, g, b, _ := c.RGBA()
	return [3]uint32{r >> 8, g >> 8, b >> 8}
}

func TestParseCSSColor(t *testing.T) {
	tests := []struct {
		in   string
		want [3]uint32
	}{
		{"lightcoral", [3]uint32{0xf0, 0x80, 0x80}},
		{"RebeccaPurple", [3]uint32{0x66, 0x33, 0x99}},
		{"#1e90ff", [3]uint32{0x1e, 0x90, 0xff}},
	}
	for _, tt := range tests {
		got, err := theme.ParseCSSColor(tt.in)
		if err != nil {
			t.Errorf("ParseCSSColor(%q): %v", tt.in, err)
			continue
		}
		if rgb(got) != tt.want {
			t.Errorf("ParseCSSColor(%q) = %x, want %x", tt.in, rgb(got), tt.want)
		}
	}
	for _, in := range []string{"", "notacolour", "12", "#f80", "#gg0000"} {
		if c, err := theme.ParseCSSColor(in); err == nil {
			t.Errorf("ParseCSSColor(%q) = %v, want an error", in, c)
		}
	}
}
