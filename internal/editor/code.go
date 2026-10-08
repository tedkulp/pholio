package editor

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"

	"github.com/tedkulp/pholio/internal/engine"
)

// fenceBlock is one fenced code block: the opening fence at line start, and
// the closing fence at line end, or end is the line count when the block
// runs unclosed to the end of the buffer.
type fenceBlock struct {
	start, end int
	lang       string   // first word of the info string; "" for none
	code       [][]kind // kinds of lines start+1..end-1, once lexed
	lexed      bool
}

// fenceLine is what the fence pass knows about one line: whether it is a
// fence or inside a block, and the syntax kinds of a code line (nil draws
// the line in flat kCode).
type fenceLine struct {
	in   bool
	code []kind
}

// fencePass is the fence pass over one buffer version. Blocks are lexed
// lazily, when a line in them is first drawn.
type fencePass struct {
	ver    uint64
	block  []int // per line: index into blocks plus one; 0 is outside
	blocks []fenceBlock
	memo   lexMemo
}

// lexMemo keeps lexed blocks by language and text across buffer versions,
// so an edit relexes only the block it changed.
type lexMemo map[string][][]kind

// maxMemo bounds the memo; it starts over when full.
const maxMemo = 512

// lexes counts lexer runs, for tests.
var lexes int

func newFencePass(b *engine.Buffer, memo lexMemo) *fencePass {
	if memo == nil || len(memo) >= maxMemo {
		memo = lexMemo{}
	}
	f := &fencePass{ver: b.Version(), block: make([]int, b.LineCount()), memo: memo}
	for i := 0; i < b.LineCount(); i++ {
		if !isFence(b.Line(i)) {
			continue
		}
		blk := fenceBlock{start: i, end: b.LineCount(), lang: fenceLang(b.Line(i))}
		for j := i + 1; j < b.LineCount(); j++ {
			if isFence(b.Line(j)) {
				blk.end = j
				break
			}
		}
		f.blocks = append(f.blocks, blk)
		for j := i; j <= min(blk.end, b.LineCount()-1); j++ {
			f.block[j] = len(f.blocks)
		}
		i = blk.end
	}
	return f
}

// fenceLang is the first word of a fence's info string.
func fenceLang(l string) string {
	info := strings.TrimLeft(strings.TrimLeft(l, " "), "`~")
	if w := strings.Fields(info); len(w) > 0 {
		return w[0]
	}
	return ""
}

// line is the fence state of line i of b, the buffer the pass was made from.
func (f *fencePass) line(b *engine.Buffer, i int) fenceLine {
	if i >= len(f.block) || f.block[i] == 0 {
		return fenceLine{}
	}
	blk := &f.blocks[f.block[i]-1]
	if i == blk.start || i == blk.end {
		return fenceLine{in: true}
	}
	if !blk.lexed {
		blk.lexed = true
		blk.code = f.lex(b, blk)
	}
	if blk.code == nil {
		return fenceLine{in: true}
	}
	return fenceLine{in: true, code: blk.code[i-blk.start-1]}
}

// lex tokenises a block's lines as one text, so that strings and comments
// spanning lines are seen whole. A block with no language, or one with no
// lexer, gives nil: the language is never guessed.
func (f *fencePass) lex(b *engine.Buffer, blk *fenceBlock) [][]kind {
	if blk.lang == "" {
		return nil
	}
	lexer := lexers.Get(blk.lang)
	if lexer == nil {
		return nil
	}
	lines := make([]string, 0, blk.end-blk.start-1)
	for i := blk.start + 1; i < blk.end; i++ {
		lines = append(lines, b.Line(i))
	}
	text := strings.Join(lines, "\n")
	key := lexer.Config().Name + "\x00" + text
	if ks, ok := f.memo[key]; ok {
		return ks
	}
	lexes++
	ks := make([][]kind, len(lines))
	for i, l := range lines {
		ks[i] = make([]kind, len(l))
		for j := range ks[i] {
			ks[i][j] = kCode
		}
	}
	it, err := chroma.Coalesce(lexer).Tokenise(nil, text+"\n")
	if err != nil {
		return nil
	}
	line, col := 0, 0
	for t := it(); t != chroma.EOF; t = it() {
		k := tokenKind(t.Type)
		for i := 0; i < len(t.Value); i++ {
			if t.Value[i] == '\n' {
				line, col = line+1, 0
				continue
			}
			if line < len(ks) && col < len(ks[line]) {
				ks[line][col] = k
			}
			col++
		}
	}
	f.memo[key] = ks
	return ks
}

// tokenKind maps a chroma token to the code kind it draws as.
func tokenKind(t chroma.TokenType) kind {
	switch {
	case t == chroma.KeywordType, t == chroma.NameClass, t.InSubCategory(chroma.NameBuiltin):
		return kType
	case t.InCategory(chroma.Keyword):
		return kKeyword
	case t.InSubCategory(chroma.LiteralString):
		return kString
	case t.InCategory(chroma.Comment):
		return kComment
	case t.InSubCategory(chroma.LiteralNumber):
		return kNumber
	case t.InSubCategory(chroma.NameFunction):
		return kFunction
	case t.InCategory(chroma.Operator):
		return kOperator
	}
	return kCode
}
