# Prior art: modal text editors in Go / Bubble Tea (as of 2026-10-05)

Answers ticket `issues/02-go-modal-editor-prior-art.md`. Every claim cites a primary source: the repository itself (source at the commit named in the table, read from a shallow clone on 2026-10-05) or the GitHub REST API (`gh api repos/<owner>/<repo>`, same day) for stars, last push, license and releases. Source references of the form `repo@sha:path:line` mean that file at that commit; the GitHub blob URL `https://github.com/<repo>/blob/<sha>/<path>#L<line>` shows the same thing.

Context from ticket 01: pholio builds on `charm.land/bubbletea/v2` v2.0.10, `charm.land/bubbles/v2` v2.2.1 and `charm.land/lipgloss/v2` v2.0.6 (`research/bubbletea-ecosystem.md`). Candidates that still target the v1 Charm stack would need porting.

## 1. Candidates at a glance

| Candidate | What it is | Stars | Last push | License | Commit read | Charm stack |
|---|---|---|---|---|---|---|
| [charmbracelet/bubbles `textarea`](https://github.com/charmbracelet/bubbles/tree/c3bde0d166/textarea) | Stock multi-line input, no modes | 8,970 | 2026-10-04 (release v2.2.1, 2026-08-24) | MIT | `c3bde0d166` | v2 |
| [ionut-t/goeditor](https://github.com/ionut-t/goeditor) | Vim editor component for Bubble Tea | 16 | 2026-08-15 (release v0.6.0) | MIT | `08a3c0cd05` | v2 |
| [kujtimiihoxha/vimtea](https://github.com/kujtimiihoxha/vimtea) | Vim editor component for Bubble Tea | 72 | 2025-03-29 | MIT | `a250e98498` | v1 |
| [guygrigsby/vimbubble](https://github.com/guygrigsby/vimbubble) | Vim layer over a bubbles `textarea` | 0 | 2026-09-09 | MIT | `60c478e367` | v1 (root) and v2 (`v2/`) |
| [mieubrisse/vim-bubble](https://github.com/mieubrisse/vim-bubble) | Vim component on a forked textarea | 4 | 2023-04-23 | none (no license file / API reports null) | `7bed1274f6` | v1 |
| [praxis-labs-io/zen-notes](https://github.com/praxis-labs-io/zen-notes) | Daily-notes app with its own vim editor (app, not a library) | 1 | 2026-10-03 | MIT | `003a74b30e` | v2 |
| [aretext/aretext](https://github.com/aretext/aretext) | Full vim-compatible editor (tcell) | 288 | 2026-09-22 (release v1.7.0, 2026-06-06) | **GPL-3.0** | `c051d09551` | none (tcell v3) |
| [micro-editor/micro](https://github.com/micro-editor/micro) | Non-modal editor (tcell fork) | 29,668 | 2026-10-05 (release v2.0.15, 2025-12-31) | MIT | `02164788bd` | none |
| [zyedidia/mu](https://github.com/zyedidia/mu) | Modal editor by micro's author (tcell) | 2 | 2026-10-03 | MIT | `2ea5abad07` | none |
| [xyproto/orbiton](https://github.com/xyproto/orbiton) | Non-modal editor | 710 | 2026-10-05 | BSD-3-Clause | `88be200fdb` | none |
| [nsf/godit](https://github.com/nsf/godit) | Emacs-ish editor | 581 | 2026-08-23 | MIT | `da0eb1a2cf` | none |
| [eugenioenko/ttt](https://github.com/eugenioenko/ttt) | Editor/IDE (tcell v3) | 388 | 2026-10-02 | MIT | `72392bac27` | none |

Stars, pushed_at and license from `gh api repos/<repo>` on 2026-10-05; release tags from `gh api repos/<repo>/releases/latest`.

A GitHub repository search for vim + bubbletea / textarea / component (`gh api search/repositories`, 2026-10-05) turned up only the four Bubble Tea vim components above plus many apps that use "vim keybindings" for list navigation only (not text editing).

## 2. Bubble Tea components that implement vim modes

### 2.1 Bubbles `textarea` (the baseline, no vim)
- **Buffer:** `value [][]rune`, one rune slice per line (`bubbles@c3bde0d166:textarea/textarea.go:361`). A hard cap `maxLines = 10000` exists, with a comment "XXX: in v2, make max lines dynamic" (`textarea.go:35-37`). Default `MaxHeight = 99` (`textarea.go:33`).
- **No undo/redo:** there is no undo or redo anywhere in `textarea.go` (grep for `undo`/`redo` returns nothing).
- **Editing primitives are unexported:** word motions and deletions (`wordLeft`, `wordRight`, `deleteWordLeft`, `characterLeft` …) are lower-case methods (`textarea.go:901-1023`, `984`). The exported surface is limited to `InsertString`, `SetValue`, `Value`, `Line`, `Column`, `SetCursorColumn`, `CursorUp/Down/Start/End`, `MoveToBegin/End`, `PageUp/Down`, and selection helpers (`textarea.go:524-1160`, `1823-1829`).
- **Soft wrap:** a word-wrap `wrap(runes, width)` (`textarea.go:2020`) memoized per line in an LRU (`memoization.MemoCache[line, [][]rune]`, `textarea.go:279`, `412`).
- **Unicode width:** cell widths come from `uniseg.StringWidth` and `go-runewidth` (`textarea.go:24-25`, `662`, `1091`, `2037`, `2053`). The cursor column `col` is a **rune** index and left/right move by one rune (`characterLeft` does `SetCursorColumn(m.col - 1)`, `textarea.go:984-995`). So a combining sequence or ZWJ emoji takes several keypresses to cross.
- **Cursor:** virtual by default (`useVirtualCursor: true`, `textarea.go:415`). With `SetVirtualCursor(false)`, `Cursor()` returns a `*tea.Cursor` with a position relative to the textarea. The parent must add the pane offset (`textarea.go:1738-1777`).
- **Upstream vim plans:** a "Vim-style editing for textarea" PR (#207, opened 2022-08-11 by a Charm maintainer) was closed unmerged on 2024-08-05 (`gh api repos/charmbracelet/bubbles/pulls/207`). Charm has no vim mode on its roadmap in the repo.

### 2.2 ionut-t/goeditor (most complete Bubble Tea vim component)
- **Status:** created 2025-04-27, ≥100 commits, 3 contributors, v0.6.0 released 2026-08-15, MIT (`gh api repos/ionut-t/goeditor`, `/contributors`, `/releases/latest`). It is on the v2 stack: `charm.land/bubbletea/v2 v2.0.8`, `bubbles/v2 v2.1.1`, `lipgloss/v2 v2.0.6` (`goeditor@08a3c0cd05:go.mod`). Known users: `danvergara/dblab` (3,241 stars) and `savimcio/nistru` import it (GitHub code search for the module path in `go.mod`; `nistru@c565f0f5ce:go.mod`).
- **Features (README):** Normal, Insert, Visual, Visual Line and Command modes, undo/redo including line-wise `U`, `:map` family plus a Go remap API, search, clipboard, word wrap, relative numbers, chroma syntax highlighting, word highlights, completion menu, and `DisableVimMode` (`goeditor@08a3c0cd05:README.md`). The source has text objects, char search (`f/t`), case ops, yank/paste, counts (`core/textobject.go`, `core/charsearch.go`, `core/case.go`, `core/yank.go`; `PendingCount *int` in `core/state.go:19-60`). Total size is about 19k lines of Go including tests.
- **Buffer:** `lines [][]rune` behind a `Buffer` interface (`core/buffer.go:10-36`, `81`).
- **Undo:** full snapshots. `SaveHistory()` stores `buffer.GetCurrentContent()` (the whole document as a string) plus the cursor (`core/state.go:884-899`).
- **Command parsing:** one `EditorMode` object per mode with `HandleKey`, plus `AwaitingLiteral()` / `OperatorPending()` flags that select mapping sets (`core/mode.go:18-36`). It is hand-written dispatch, not a grammar or DFA.
- **Unicode / wrap:** grapheme clusters and widths come from `uniseg` in the renderer (`visual_layout.go:12`, `28-37`, `66-97`), with word-wrap at `visual_layout.go:979-1006`. Cursor motions are still **rune**-indexed (`core/cursor.go:54-64`: `Col++`).
- **Cursor:** drawn as a styled cell (`getCursorStyles().Render(...)`, `visual_layout.go:771`, `796`, `1052`). It never sets `tea.Cursor`, so there is no real terminal cursor (no `NewCursor`/`tea.Cursor` anywhere in the module).

### 2.3 kujtimiihoxha/vimtea
- MIT, 72 stars, a single contributor, last push 2025-03-29, 6 open issues. It is on the **v1** stack (`github.com/charmbracelet/bubbletea v1.3.4`, `bubbles v0.20.0`) (`vimtea@a250e98498:go.mod`; `gh api`). A downstream fork exists (`jleija-lumino/vimtea`, 2026-08-18), which suggests upstream is dormant.
- Buffer `lines []string` with full-snapshot undo/redo stacks (`buffer.go:53-61`). Normal/Insert/Visual/Command modes and counts (README). Cursor rendered as a styled cell (`view.go:127-158`). No soft wrap code in `view.go`.

### 2.4 guygrigsby/vimbubble
- MIT, 0 stars, one commit, 2026-09-09. It has a v1 module at the root and a separate `v2/` module on `charm.land/bubbles/v2 v2.2.1` (`vimbubble@60c478e367:README.md`, `v2/go.mod`).
- It is a key translator: NORMAL keys are rewritten into textarea key events (`h` becomes `KeyLeft`, `w` becomes `Alt+f`). Verbs the textarea lacks are done with `Value()` → split → edit → `SetValue()` round-trips (`v2/cursor.go:43-64`). The v1 module reads the cursor through `reflect`+`unsafe` because bubbles v1 did not export it (README "How it works").
- It says outright that it lacks undo/redo, registers, search, `f/t`, counts and more (README "What's not (yet) here"; `v2/doc.go:117-119`). This shows the ceiling of layering vim over `textarea`.

### 2.5 mieubrisse/vim-bubble
- No license, 4 stars, one commit, last push 2023-04-23, v1 stack (`gh api`; `vim-bubble@7bed1274f6:README.md`). It is a "heavily-modified fork" of the textarea with 20-step snapshot undo (`vim/vim.go:28`). It is unusable as a dependency because it has no license. It is mentioned only as an example of forking `textarea`.

### 2.6 praxis-labs-io/zen-notes (closest product analogue)
- A daily-notes app ("One notepad per day … edited with vim motions"), MIT, created 2026-08-17 (`zen-notes@003a74b30e:README.md`; `gh api`). It is not a library because the editor is under `internal/`.
- Design worth copying: buffer `lines [][]rune` (`internal/editor/buffer.go:23-25`), an engine (`internal/editor/vim.go`, 1.3k lines) separate from rendering (`render.go`), and a **real** terminal cursor via `tea.NewCursor`, with `CursorBar` in insert/command and `CursorBlock` in normal (`internal/app/view.go:51`, `82-91`).

## 3. Pure-Go editors and their buffer data structures

| Editor | Structure | Source |
|---|---|---|
| micro | `LineArray{ lines []Line }`, each `Line{ data []byte, highlight state… }` | `micro@02164788bd:internal/buffer/line_array.go:46-77` |
| aretext | `text.Tree`: a B+-tree rope over UTF-8 with per-node char and line counts, nodes sized to 64-byte cache lines (cites Boehm et al. 1995 "Ropes" and Rao & Ross 2000) | `aretext@c051d09551:text/tree.go:15-25` |
| mu | Byte rope with `SplitLen 4096 / JoinLen 2048` plus a line cache | `mu@2ea5abad07:text/rope.go`, `text/linecache.go`; README "performant rope data structure" |
| orbiton | `lines map[int][]rune` | `orbiton@88be200fdb:v2/editor.go:50` |
| godit | Doubly linked list of `line{ data []byte }` | `godit@da0eb1a2cf:buffer.go:16-20`, `45-55` |
| ttt | `Lines []string` | `ttt@72392bac27:internal/core/buffer/buffer.go:72-73` |
| goeditor / zen-notes / textarea | `[][]rune` | §2 |
| vimtea | `[]string` | §2.3 |

Standalone Go rope and piece-table packages are all small and mostly abandoned. Examples: `zyedidia/rope` (8 stars, last push 2021-06-16), `vinzmay/go-rope` (2014), `chewxy/skiprope` (2017). On the piece-table side, `gboncoffee/gopiecetable` is the only one pushed in 2026 and has 1 star (`gh api search/repositories`, "rope data structure language:go" / "piece table language:go", 2026-10-05).

**Trade-offs for note-sized files (a few KB to a few hundred KB):**
- **Line slice (`[][]rune` / `[]string` / `[]Line`).** Insert or delete inside a line costs O(line length). Splitting or joining lines costs O(number of lines) to shift slice headers. Line N is O(1), which is what a line-oriented renderer, `j/k`, line numbers and soft-wrap caches want. Most editors in the table use it, including micro, the one with by far the most users. It is the obvious fit for notes.
- **Rope / B-tree (aretext, mu).** O(log n) edits and offset↔line lookups. It pays off for multi-MB files and makes every "give me line N" go through a tree walk or cache. mu adds a separate `linecache.go` for this. aretext's tree is built for cache-line efficiency (`tree.go:21-24`). That complexity buys nothing at note size.
- **Piece table.** Cheap undo (append-only add buffer). It has no maintained Go implementation (above), and the undo benefit can be had more simply (next point).
- **Undo is independent of the buffer.** aretext logs inverse-able `Op{pos, insertText, deleteText}` (`aretext@c051d09551:undo/op.go`). micro keeps a `TextEvent` undo stack (`micro@02164788bd:internal/buffer/eventhandler.go:29`, `145-156`). goeditor and vimtea snapshot the whole document (§2.2, §2.3). For notes, whole-document snapshots are also viable. A 50 KB note × 100 undo steps = 5 MB worst case, at the cost of no fine-grained cursor restore.
- **Rune vs byte storage.** `[][]rune` makes column math trivial but uses 4× the memory of UTF-8 bytes. micro and aretext store bytes and decode on the fly. At note size the memory difference does not matter.

## 4. Unicode width, soft wrap and cursor rendering

### 4.1 Width libraries in play
- `mattn/go-runewidth`: per-rune East-Asian width. MIT, last push 2026-09-24, tag v0.0.30 (`gh api`). micro uses it (`micro@02164788bd:go.mod:9`, `internal/util/util.go:186`, `223`), as do mu (`go.mod`) and orbiton.
- `rivo/uniseg`: grapheme clusters + `StringWidth`. MIT, last push **2024-05-31**, so it looks stale (`gh api repos/rivo/uniseg`). bubbles textarea and goeditor use it. mu vendors a fork, `github.com/zyedidia/uniseg` (`mu@2ea5abad07:go.mod`).
- `clipperhouse/displaywidth` + `clipperhouse/uax29`: newer grapheme-aware width library, MIT (`gh api`). `charmbracelet/x/ansi` uses it (`x@ad85c59fdf:ansi/method.go:7`). So do aretext (`aretext@c051d09551:go.mod`; copied tcell's `RUNEWIDTH_EASTASIAN` handling, `cellwidth/cellwidth.go:13-18`) and ttt (`go.mod`).
- `charmbracelet/x/ansi` exposes two width methods, `WcWidth` (per-rune, wcwidth-style) and `GraphemeWidth`, plus `StringWidth`, `Truncate`, `Wrap`, `Wordwrap` and `Hardwrap`, all ANSI-aware (`x@ad85c59fdf:ansi/method.go:25-45`, `ansi/width.go:66-75`, `ansi/wrap.go:21`, `128`, `276`). This is what Lip Gloss v2 and Bubble Tea v2 use, so pholio should measure with the same library the renderer uses.

### 4.2 Grapheme width is terminal-dependent, and Bubble Tea v2 negotiates it
- At startup Bubble Tea v2 queries DEC mode 2027 (`RequestModeUnicodeCore`). If the terminal reports it, the renderer switches from the default `WcWidth` to `GraphemeWidth` and enables the mode (`bubbletea@96d69d2f7e:tea.go:803-806`, `1121-1125`; `cursed_renderer.go:740-752`). So the cell width of an emoji ZWJ sequence differs between terminals. An editor that computes cursor columns must use **the same method the renderer is using**, or the real cursor drifts.
- aretext states the same problem in its docs: terminal glyph widths can disagree with the computed width and cause "misaligned cursor position". It offers a `showUnicode` escape mode as a workaround (`aretext@c051d09551:docs/unicode.md`).
- micro treats a "character" as a rune plus following combining marks (`isMark`), not as a full UAX #29 grapheme cluster (`micro@02164788bd:internal/util/unicode.go:31-46`).
- Only aretext moves the cursor by **grapheme cluster** (`locate.NextCharInLine` iterates `segment.NewGraphemeClusterIter`, `aretext@c051d09551:locate/character.go:11-19`; own UAX #29 tables in `text/segment/`). textarea and goeditor move by rune (§2.1, §2.2).

### 4.3 Soft wrap
- micro keeps soft wrap entirely in the view layer. `SLoc{Line, Row}` addresses a visual row inside a buffer line, and `VLocFromLoc` maps buffer positions to visual positions (`micro@02164788bd:internal/display/softwrap.go:10-15`, `70`, `322`). The buffer knows nothing about wrapping.
- aretext does the same with a `WrappedLineIter` over the tree, configured by width and a cell-width sizer (`aretext@c051d09551:display/buffer.go:35-78`).
- textarea and goeditor wrap per logical line and cache the result (textarea's LRU memo, §2.1; goeditor `visual_layout.go:979-1006`). goeditor's `MoveRightOrDown` has to special-case "visually at end of a wrapped segment" (`core/cursor.go:205-212`). Wrap handling leaks into motions when the engine is not designed around `SLoc`-style visual addressing.

### 4.4 Cursor rendering
- Real terminal cursor: micro (`screen.ShowCursor`, with a reverse-video fake cursor only where needed, e.g. Windows console or multi-cursor, `micro@02164788bd:internal/screen/screen.go:95-113`), aretext (`sr.ShowCursor`, `display/buffer.go:206`), mu (`editor.go:1220`), zen-notes (`tea.NewCursor` with shape per mode, §2.6).
- Fake (styled-cell) cursor: goeditor, vimtea, and textarea's default virtual cursor (§2).
- Under Bubble Tea v2 a real cursor is a `*tea.Cursor` field on `tea.View` (position, color, `Shape`, `Blink`) (`bubbletea@96d69d2f7e:tea.go:84-140`, `365-385`). A real cursor gives native block/bar shape changes per mode (as zen-notes does) and stays correct under terminal-specific grapheme widths only if the column is computed with the renderer's width method (§4.2).

## 5. Reusable vim keymap / state-machine packages

- **No standalone Go package exists** that parses vim key sequences (count, register, operator, motion, text object) independently of an editor. GitHub repository searches for vim motions, vi keybinding parser, vim emulation and modal editor in Go (2026-10-05) returned only full editors, apps, and the Bubble Tea components in §2.
- **aretext's input engine is the best-designed one.** Each `Command` has a `BuildExpr` (a regular expression over key events: `EventExpr`, `EventRangeExpr`, `ConcatExpr`, …) and a `BuildAction(ctx, CommandParams{Count, ClipboardPage, MatchChar, ReplaceChar, InsertChar})`. All commands per mode compile into a DFA (`StateMachine` with `transitions` and `acceptCmd`), serialized to `input/generated/*.bin` via `go:generate` (`aretext@c051d09551:input/commands.go:1-80`, `input/engine/statemachine.go`, `input/engine/expression.go`, `input/interpreter.go:170`). Counts are capped at 1024 (`commands.go`, `defaultMaxCount`). However, it is **GPL-3.0** (`LICENSE`), tied to `tcell` key types and aretext's `state.EditorState`. Depending on it or copying code would make pholio GPL. Studying the design is fine.
- **goeditor's `core` package** is the only MIT, v2-stack, importable vim engine with real coverage (text objects, `f/t`, counts, visual modes, `:map`, undo). Drawbacks for pholio: full-snapshot undo, rune-indexed motions, a fake cursor, a single main maintainer with 16 stars, and an API shaped around its own `Model`/`View` and chroma highlighting rather than pholio's Note/Link rendering (§2.2).
- **vimbubble / textarea layering** is capped by the textarea's unexported internals and lack of undo (§2.1, §2.4).
- Vim's own grammar is `[count]["x]operator[count]motion` or text object (`:help operator`, `:help text-objects`, https://vimhelp.org/motion.txt.html#operator). A small parser for that shape (pending count, pending register, pending operator, then a motion or object, plus literal-arg states for `f/t/r`) is a few hundred lines. Both goeditor's `OperatorPending`/`AwaitingLiteral` flags and aretext's DFA model the same shape.

## Recommendation for pholio

1. **Write our own engine; do not depend on a vim package.** Nothing reusable exists outside full editors. The only MIT/v2 candidate (goeditor) is a whole UI component with snapshot undo, a fake cursor and rune-level motions, which pholio would fight when rendering Links and Tasks inline. Keep goeditor (MIT) open as a **reference and source of test cases** for motion and text-object semantics. Copying small MIT pieces with attribution is allowed. Study aretext's command-as-expression and DFA design but **copy no aretext code (GPL-3.0)**.
2. **Buffer: `[]string` or `[][]rune` per line**, behind a small interface (insert/delete at a position, line N, line count) so it could be swapped later. Ropes and piece tables only pay off on multi-MB files (§3).
3. **Undo: an op log of `{pos, inserted, deleted}` with cursor before/after**, grouped per normal-mode command or insert session (aretext/micro style), rather than whole-text snapshots. Snapshots are an acceptable shortcut for the prototype (ticket 03).
4. **Parser: explicit pending-state machine** for `count → register → operator → count → motion/text-object`, with literal-arg states for `f t r`. Commands are data (name, key pattern, action) so keymaps stay configurable (ties to ticket 08).
5. **Unicode: measure with `charmbracelet/x/ansi`** (the renderer's own library) and honour the width method Bubble Tea negotiated via mode 2027. Make `h/l`/`x` operate on **grapheme clusters** (x/ansi already depends on `clipperhouse/uax29` for segmentation), not runes.
6. **Soft wrap in the view only**, micro-style: the engine works in `(line, col)` and the view maps to `(line, row, cell)`. `gj`/`gk` and scrolling ask the view for that mapping.
7. **Real cursor:** emit a `*tea.Cursor` from `View()` with block in normal mode and bar in insert, as zen-notes does. Compute its column from the same width function as the rendered text.
