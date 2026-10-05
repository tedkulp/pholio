// PROTOTYPE — throwaway code for wayfinder ticket "Vim editor prototype".

package main

import (
	"regexp"
	"strings"
)

// Style ids, one per byte of a line. Rendering groups runs of equal ids.
const (
	sNone uint8 = iota
	sHeading
	sMarker // dim syntax: **, [[, ](url), #, >, fences
	sBold
	sItalic
	sCode
	sLink
	sTaskBox
	sTaskDone
	sTag
	sMeta
	sQuote
	sBullet
	sSearch
	sVisual
	sCount
)

var (
	reCode   = regexp.MustCompile("`[^`]+`")
	reWiki   = regexp.MustCompile(`\[\[([^\]]+)\]\]`)
	reMdLink = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	reBold   = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	reItalic = regexp.MustCompile(`(?:^|[^*\w])([*_])([^*_\s][^*_]*)([*_])`)
	reTag    = regexp.MustCompile(`(?:^|\s)(#[\p{L}\d_/-]+)`)
	reMeta   = regexp.MustCompile(`(?:^|\s)([a-z]+:[^\s:]+)`)
	reHead   = regexp.MustCompile(`^(#{1,6})\s`)
	reList   = regexp.MustCompile(`^(\s*)([-*+]|\d+[.)])\s+(\[([ xX])\]\s)?`)
	reQuote  = regexp.MustCompile(`^\s*>`)
	reHR     = regexp.MustCompile(`^\s*(\*\*\*+|---+|___+)\s*$`)
)

func isFence(l string) bool {
	t := strings.TrimLeft(l, " ")
	return strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~")
}

// fenceStates[i] reports whether line i is inside a fenced code block.
// One pass of HasPrefix over every line above the viewport: the only
// cross-line state markdown highlighting needs.
func fenceStates(lineAt func(int) string, upto int) []bool {
	st := make([]bool, upto)
	in := false
	for i := 0; i < upto; i++ {
		if isFence(lineAt(i)) {
			st[i] = true
			in = !in
			continue
		}
		st[i] = in
	}
	return st
}

// highlight returns a style id and a conceal flag per byte.
func highlight(l string, inFence bool) (st []uint8, conceal []bool) {
	st = make([]uint8, len(l))
	conceal = make([]bool, len(l))
	fill := func(a, z int, s uint8) {
		for i := a; i < z; i++ {
			st[i] = s
		}
	}
	hide := func(a, z int) {
		fill(a, z, sMarker)
		for i := a; i < z; i++ {
			conceal[i] = true
		}
	}
	if isFence(l) {
		fill(0, len(l), sMarker)
		return
	}
	if inFence {
		fill(0, len(l), sCode)
		return
	}
	if reHR.MatchString(l) {
		fill(0, len(l), sMarker)
		return
	}
	if m := reHead.FindStringSubmatchIndex(l); m != nil {
		fill(0, len(l), sHeading)
		fill(m[2], m[3], sMarker)
	}
	if m := reQuote.FindStringIndex(l); m != nil {
		fill(0, len(l), sQuote)
		fill(m[0], m[1], sMarker)
	}
	task := false
	if m := reList.FindStringSubmatchIndex(l); m != nil {
		fill(m[4], m[5], sBullet)
		if m[6] >= 0 {
			task = true
			if l[m[8]] == ' ' {
				fill(m[6], m[7], sTaskBox)
			} else {
				fill(m[6], len(l), sTaskDone)
				return
			}
		}
	}
	locked := make([]bool, len(l)) // inside `code`: nothing else applies
	for _, m := range reCode.FindAllStringIndex(l, -1) {
		fill(m[0], m[1], sCode)
		hide(m[0], m[0]+1)
		hide(m[1]-1, m[1])
		for i := m[0]; i < m[1]; i++ {
			locked[i] = true
		}
	}
	free := func(a, z int) bool {
		for i := a; i < z; i++ {
			if locked[i] {
				return false
			}
		}
		return true
	}
	for _, m := range reWiki.FindAllStringSubmatchIndex(l, -1) {
		if free(m[0], m[1]) {
			fill(m[2], m[3], sLink)
			hide(m[0], m[2])
			hide(m[3], m[1])
		}
	}
	for _, m := range reMdLink.FindAllStringSubmatchIndex(l, -1) {
		if free(m[0], m[1]) {
			fill(m[2], m[3], sLink)
			hide(m[0], m[2])
			hide(m[3], m[1])
		}
	}
	for _, m := range reBold.FindAllStringSubmatchIndex(l, -1) {
		if free(m[0], m[1]) {
			fill(m[2], m[3], sBold)
			hide(m[0], m[2])
			hide(m[3], m[1])
		}
	}
	for _, m := range reItalic.FindAllStringSubmatchIndex(l, -1) {
		if free(m[2], m[7]) && l[m[2]] == l[m[6]] {
			fill(m[4], m[5], sItalic)
			hide(m[2], m[3])
			hide(m[6], m[7])
		}
	}
	for _, m := range reTag.FindAllStringSubmatchIndex(l, -1) {
		if free(m[2], m[3]) && st[m[2]] != sHeading {
			fill(m[2], m[3], sTag)
		}
	}
	if task {
		for _, m := range reMeta.FindAllStringSubmatchIndex(l, -1) {
			if free(m[2], m[3]) {
				fill(m[2], m[3], sMeta)
			}
		}
	}
	return
}
