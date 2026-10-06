package engine

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
	"github.com/clipperhouse/uax29/v2/graphemes"
)

// TabStop is the tab width in cells.
const TabStop = 4

// nextG returns the byte index of the grapheme after the one at i.
func nextG(s string, i int) int {
	if i >= len(s) {
		return len(s)
	}
	it := graphemes.FromString(s[i:])
	it.Next()
	return i + it.End()
}

// prevG returns the byte index of the grapheme before i.
func prevG(s string, i int) int {
	prev := 0
	it := graphemes.FromString(s)
	for it.Next() {
		if it.Start() >= i {
			break
		}
		prev = it.Start()
	}
	return prev
}

// lastG is the start of the final grapheme (0 for an empty line).
func lastG(s string) int { return prevG(s, len(s)) }

// CellWidth is the display width of grapheme g when it starts at screen
// cell col. The engine (for j/k column memory) and the view (for layout and
// the cursor) both use this one function so they cannot disagree.
func CellWidth(g string, col int) int {
	if g == "\t" {
		return TabStop - col%TabStop
	}
	return ansi.StringWidth(g)
}

// Cells is the display width of s[:byteCol].
func Cells(s string, byteCol int) int {
	w := 0
	it := graphemes.FromString(s[:min(byteCol, len(s))])
	for it.Next() {
		w += CellWidth(it.Value(), w)
	}
	return w
}

// colAtCells finds the byte column of the grapheme covering screen cell c.
func colAtCells(s string, c int) int {
	w := 0
	it := graphemes.FromString(s)
	for it.Next() {
		gw := CellWidth(it.Value(), w)
		if w+gw > c {
			return it.Start()
		}
		w += gw
	}
	return len(s)
}

func runeAt(s string, i int) rune {
	if i >= len(s) {
		return '\n'
	}
	r, _ := utf8.DecodeRuneInString(s[i:])
	return r
}

// Character classes for word motions.
const (
	clsBlank = iota
	clsPunct
	clsWord
)

func class(r rune, big bool) int {
	switch {
	case r == ' ' || r == '\t' || r == '\n':
		return clsBlank
	case big:
		return clsWord
	case r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) || r >= 0x2000:
		return clsWord
	}
	return clsPunct
}

func firstNonBlank(s string) int { return len(s) - len(strings.TrimLeft(s, " \t")) }

func isBlankLine(s string) bool { return strings.TrimSpace(s) == "" }
