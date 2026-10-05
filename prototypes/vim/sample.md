# 2026-10-05 Sunday

Morning. Picked up the thread from [[2026-10-04]] about the editor engine. The open question is whether a hand-rolled vim engine *feels* right inside a TUI, or whether every small mismatch with real vim becomes a papercut you notice forty times a day.

## Tasks

- [ ] call bob about the cabin due:2026-10-10 pri:high #home
- [ ] write up the **vim prototype** findings #pholio
- [x] renew passport due:2026-09-30 #admin
- [ ] read [Ropes: an Alternative to Strings](https://example.com/ropes.pdf) #reading
  - [ ] nested task, indented two spaces

## Notes

> Quote: "The cheapest prototype that answers the question is the right one."

Some `inline code`, some **bold text**, some _italic text_ and a [[Zettel Link]] in the middle of a sentence. Here is a very long line that should soft-wrap at the edge of the terminal so we can feel how j and k and the cursor behave when a single logical line spans several screen rows; does it feel natural or does it fight you?

1. numbered item one
2. numbered item two (press `o` here: the list continues)

```go
func main() {
	fmt.Println("fenced code is highlighted as one block") // # not a heading
}
```

Unicode: naïve café — 日本語のテキスト — emoji 👍🏽 and 👨‍👩‍👧 — combining é (e + U+0301).
Tabs:	one	two	three

---

### Text objects to try

- inside parens (cursor here then `ci(`) and [brackets] and {braces}
- "double quoted" and 'single quoted' strings
- **bold words** for `ci*` and `da*`

Last paragraph. Try `dap`, `yip`, `}` and `{` here.
