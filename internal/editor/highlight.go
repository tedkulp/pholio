package editor

import (
	"regexp"
	"strings"

	"github.com/tedkulp/pholio/internal/engine"
	"github.com/tedkulp/pholio/internal/theme"
)

// kind is what a byte of a line is, as markdown. Each kind draws through
// one markdown theme slot.
type kind uint8

const (
	kText kind = iota
	kHeading
	kMarker // syntax: **, ~~, [[ ]], ](url), #, >, fences
	kBold
	kItalic
	kStrike
	kCode
	kLink
	kBullet
	kQuote
	kTaskBox
	kTaskDone
	kTag
	kMeta
	kinds
)

var kindSlots = [kinds]theme.Slot{
	kText:     theme.UIBase,
	kHeading:  theme.MarkdownHeading,
	kMarker:   theme.MarkdownMarker,
	kBold:     theme.MarkdownBold,
	kItalic:   theme.MarkdownItalic,
	kStrike:   theme.MarkdownStrike,
	kCode:     theme.MarkdownCode,
	kLink:     theme.MarkdownLink,
	kBullet:   theme.MarkdownBullet,
	kQuote:    theme.MarkdownQuote,
	kTaskBox:  theme.MarkdownTaskBox,
	kTaskDone: theme.MarkdownTaskDone,
	kTag:      theme.MarkdownTag,
	kMeta:     theme.MarkdownMeta,
}

var (
	reCode   = regexp.MustCompile("`[^`]+`")
	reWiki   = regexp.MustCompile(`\[\[([^\]]+)\]\]`)
	reMdLink = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	reBold   = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	reStrike = regexp.MustCompile(`~~([^~]+)~~`)
	reItalic = regexp.MustCompile(`(?:^|[^*\w])([*_])([^*_\s][^*_]*)([*_])`)
	reTag    = regexp.MustCompile(`(?:^|\s)(#[\p{L}\d_/-]+)`)
	reMeta   = regexp.MustCompile(`(?:^|\s)([a-z]+:[^\s:]+)`)
	reHead   = regexp.MustCompile(`^(#{1,6})\s`)
	reList   = regexp.MustCompile(`^(\s*)([-*+]|\d+[.)])\s+(\[([ xX/-])\](?:\s|$))?`)
	reQuote  = regexp.MustCompile(`^\s*>`)
	reHR     = regexp.MustCompile(`^\s*(\*\*\*+|---+|___+)\s*$`)
)

func isFence(l string) bool {
	t := strings.TrimLeft(l, " ")
	return strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~")
}

// fences reports, per line, whether it is a fence line or inside a fenced
// code block: the only state markdown highlighting carries across lines.
func fences(b *engine.Buffer) []bool {
	st := make([]bool, b.LineCount())
	in := false
	for i := range st {
		if isFence(b.Line(i)) {
			st[i] = true
			in = !in
			continue
		}
		st[i] = in
	}
	return st
}

// highlight gives the kind of every byte of line l and which bytes conceal
// hides. inFence is true for fence lines and the lines between them.
func highlight(l string, inFence bool) (ks []kind, hidden []bool) {
	ks = make([]kind, len(l))
	hidden = make([]bool, len(l))
	fill := func(a, z int, k kind) {
		for i := a; i < z; i++ {
			ks[i] = k
		}
	}
	hide := func(a, z int) {
		fill(a, z, kMarker)
		for i := a; i < z; i++ {
			hidden[i] = true
		}
	}
	switch {
	case inFence && isFence(l), reHR.MatchString(l):
		fill(0, len(l), kMarker)
		return
	case inFence:
		fill(0, len(l), kCode)
		return
	}
	if m := reHead.FindStringSubmatchIndex(l); m != nil {
		fill(0, len(l), kHeading)
		fill(m[2], m[3], kMarker)
	}
	if m := reQuote.FindStringIndex(l); m != nil {
		fill(0, len(l), kQuote)
		fill(m[0], m[1], kMarker)
	}
	task := false
	if m := reList.FindStringSubmatchIndex(l); m != nil {
		fill(m[4], m[5], kBullet)
		if m[6] >= 0 {
			task = true
			switch l[m[8]] {
			case ' ', '/':
				fill(m[6], m[6]+3, kTaskBox)
			default: // done or cancelled
				fill(m[6], len(l), kTaskDone)
				return
			}
		}
	}
	locked := make([]bool, len(l)) // inside `code`: nothing else applies
	for _, m := range reCode.FindAllStringIndex(l, -1) {
		fill(m[0], m[1], kCode)
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
	// wrapped styles the inner group and hides the rest of each match.
	wrapped := func(re *regexp.Regexp, k kind) {
		for _, m := range re.FindAllStringSubmatchIndex(l, -1) {
			if free(m[0], m[1]) {
				fill(m[2], m[3], k)
				hide(m[0], m[2])
				hide(m[3], m[1])
			}
		}
	}
	wrapped(reWiki, kLink)
	wrapped(reMdLink, kLink)
	wrapped(reStrike, kStrike) // before bold, so bold inside it wins
	wrapped(reBold, kBold)
	for _, m := range reItalic.FindAllStringSubmatchIndex(l, -1) {
		if free(m[2], m[7]) && l[m[2]] == l[m[6]] {
			fill(m[4], m[5], kItalic)
			hide(m[2], m[3])
			hide(m[6], m[7])
		}
	}
	for _, m := range reTag.FindAllStringSubmatchIndex(l, -1) {
		if free(m[2], m[3]) && ks[m[2]] != kHeading {
			fill(m[2], m[3], kTag)
		}
	}
	if task {
		for _, m := range reMeta.FindAllStringSubmatchIndex(l, -1) {
			if free(m[2], m[3]) {
				fill(m[2], m[3], kMeta)
			}
		}
	}
	return ks, hidden
}
