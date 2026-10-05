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
