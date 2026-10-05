# PROTOTYPE: vim editor engine (throwaway)

Answers wayfinder ticket [Vim editor prototype](https://github.com/tedkulp/pholio/issues/4).
This lives on the `prototype/vim-editor` branch only. Do not merge it.

## Run

```sh
cd prototypes/vim
go run .                 # edits a scratch copy of sample.md in $TMPDIR
go run . path/to/note.md # edits a real file (:w writes it)
go run . -big 500        # sample x500 (19.5k lines), for the perf check
go test ./engine         # 69 key-sequence cases, no Bubble Tea involved
```

## Things to try

- Motions: `w b e ge W B E 0 ^ $ gg G f t F T ; , { } % n N ctrl+d ctrl+u`, all with counts
- Operators: `d c y` × any motion or text object, plus `dd cc yy x X s S D C Y r J ~ p P`
- Text objects: `iw aw iW aW ip ap i( a( i[ a[ i{ a{ i< i" i' i\``, plus markdown `i*`/`a*` (inside `**bold**`) and `i_`
- `u`, `ctrl+r`, `.` (`dw..`, `cwfoo<esc>w.`, `2.`)
- `v`, `V`, `o`, and `viw` to grow the selection; `/` `?` search (smartcase regex), including `d/foo<enter>`
- `:w :q :q! :wq :x :e file :e! :<n>`
- `:set conceal` hides `[[ ]]`, `**`, backticks and link URLs on every line except the cursor's
- `:set nowrap`, `:set relnum`, `:set nohl` (highlighting off), `:set nolistcont`, `:noh`
- In insert mode on a list or task line, `enter` / `o` continue the bullet or `- [ ]`. `enter` on an empty item ends the list.
- The status line shows the frame render time and the highlighting share of it

## Findings

See the resolution comment on the ticket.
