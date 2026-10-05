# PROTOTYPE: app shell (throwaway)

Answers wayfinder ticket [App shell prototype](https://github.com/tedkulp/pholio/issues/5).
Lives on the `prototype/app-shell` branch only. Do not merge it.
The editor pane is the real engine from `../vim` (no wrap or conceal here; those are already decided).

## Run

```sh
cd prototypes/shell
go run .              # scratch copy of ./vault in $TMPDIR, wiped each run
go run . ~/notes      # a real vault (:w writes for real)
```

## Switch things

| Key | Does |
|---|---|
| `F5` / `F6` | previous / next shell variant |
| `F7` | next theme (`default`, `ansi16`, `light`) |
| `F8` | reload the current theme from `themes/*.toml` (edit it while running) |
| `ctrl+q` | quit |

## The three variants

| | Sidebar | Focus | Task List / Zettel prompt |
|---|---|---|---|
| **A · Split + ctrl-w** | always visible, 30 cols | `ctrl+w h/l/w` (vim windows), `ctrl+w e` toggles, `ctrl+w < > =` width | centered modal over a dimmed backdrop |
| **B · Drawer** | hidden; `spc e` slides it over the editor, opening a note closes it | no focus keys: the drawer has focus while open, `esc` closes | Task List is a right-side panel; Zettel title is a vim-style prompt on the bottom line |
| **C · Tab + palette** | always visible | `tab` toggles, `ctrl+h` / `ctrl+l` | top-anchored palette; typing filters the Task List |

Shared by all: `spc t` Task List, `spc z` new Zettel (link inserted at the cursor, Origin link in the new file).
Leader keys only fire in normal mode with nothing pending, or from the sidebar.

Sidebar: `j k` move, `l`/`enter` expand or open, `h` collapse or jump to parent, `.` toggle dotfiles,
`< >` width, `g G` top/bottom, `esc` back to the editor. Folders first, `.md` shown without the extension, other files dimmed.

## Theme file

`themes/default.toml` lists every slot. A theme is a `[palette]` of named colours plus style slots in five
sections: `ui`, `sidebar`, `overlay`, `markdown`, `tasks`. Other themes only override what they change:
`light.toml` is just a palette swap. Unknown slots or keys show as an error on the message line.
