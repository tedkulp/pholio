package theme

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"golang.org/x/image/colornames"
)

// ParseColor parses a literal colour: "#rrggbb", "#rgb" or an ANSI number
// from 0 to 255. Palette names are resolved before this is called.
func ParseColor(s string) (color.Color, error) {
	if len(s) > 0 && s[0] == '#' {
		if (len(s) != 7 && len(s) != 4) || !isHex(s[1:]) {
			return nil, fmt.Errorf("bad colour %q", s)
		}
		return lipgloss.Color(s), nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 || n > 255 || s[0] == '+' {
		return nil, fmt.Errorf("bad colour %q", s)
	}
	return lipgloss.Color(s), nil
}

// ParseCSSColor parses a CSS colour: a name such as "lightcoral" (any
// case) or "#rrggbb". Calendar colours are written this way.
func ParseCSSColor(s string) (color.Color, error) {
	name := strings.ToLower(s)
	if c, ok := colornames.Map[name]; ok {
		return c, nil
	}
	if name == "rebeccapurple" { // CSS 4; colornames stops at SVG 1.1
		return color.RGBA{0x66, 0x33, 0x99, 0xff}, nil
	}
	if len(s) == 7 && s[0] == '#' && isHex(s[1:]) {
		return lipgloss.Color(s), nil
	}
	return nil, fmt.Errorf("bad colour %q: want a CSS colour name or #rrggbb", s)
}

func isHex(s string) bool {
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return false
		}
	}
	return true
}
