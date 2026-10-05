package theme

import (
	"fmt"
	"image/color"
	"strconv"

	"charm.land/lipgloss/v2"
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

func isHex(s string) bool {
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return false
		}
	}
	return true
}
