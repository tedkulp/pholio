# pholio

A terminal markdown editor for a folder of notes, with a vim-style editor built in.

pholio opens a **Vault**, a plain folder of `.md` files, and gives you a file tree, a modal editor, and the tools for a daily-notes workflow: Daily Notes, a Vault-wide Task List, `[[wikilinks]]` with Backlinks, and quick Zettel creation. There's no database. The index is rebuilt in memory on every start, and syntax stays Obsidian-compatible, so the same Vault opens fine in other tools.

```
 notes                       │# 2026-10-05
 ▾ daily                     │
     2026-10-05              │## Tasks
 ▸ zettel                    │- [ ] Review Project Alpha due:2026-10-07
   Project Alpha             │- [x] Ship v1 done:2026-10-05
                             │~
 NORMAL  daily/2026-10-05.md                                          1:1
```

## Install

pholio needs Go 1.27 or newer. It runs on Linux and macOS; Windows is not supported.

```sh
go install github.com/tedkulp/pholio/cmd/pholio@latest
```

Or build from a clone with [just](https://github.com/casey/just):

```sh
git clone https://github.com/tedkulp/pholio
cd pholio
just build        # writes bin/pholio
```

## Quick start

```sh
pholio ~/notes
```

pholio opens today's Daily Note (`daily/YYYY-MM-DD.md`), creating it if needed. Press `spc` in normal mode to see the leader keys, and `ctrl+q` to quit.

To skip the path argument, set your Vault once in `~/.config/pholio/config.toml`:

```toml
vault = "~/notes"
```

`pholio <file>` opens a single Note instead.

## Keys

The editor is vim: counts, motions, `d`/`c`/`y`, text objects (including markdown `i*` and `i_`), visual modes, undo/redo, `.`, `/` search, and `:w :q :wq :q! :e :e!`. There are no macros or `:s`.

| Key | Action | Command |
|---|---|---|
| `spc d` | today's Daily Note | `:today` |
| `[d` / `]d` | previous / next existing Daily Note | |
| `spc D` | jump to a date (ISO or `tomorrow`, `fri`, `+2w`…) | `:daily <date>` |
| `spc t` | Task List | `:tasks` |
| `spc f` | find a Note | `:find [query]` |
| `spc /` | search the Vault | `:grep [query]` |
| `spc b` | Backlinks to this Note | `:backlinks` |
| `spc z` | new Zettel, linked from here | `:zettel [title]` |
| `spc n` | new Note | `:new <name>` |
| | rename / delete this Note | `:rename <path>`, `:delete` |
| `spc e` | toggle the sidebar | `:sidebar` |
| `enter` / `gd` | follow the Link under the cursor | |
| `ctrl+o` / `tab` | jump back / forward | |
| `ctrl+h` / `ctrl+l` | focus sidebar / editor | |
| `F7` / `F8` | cycle theme / reload config and theme | |
| `ctrl+q` | quit (asks if there are unsaved changes) | `:q` |

In the sidebar, `a` adds a Note (end the name with `/` for a folder), `r` renames or moves, `d` deletes to the system trash, `.` shows dotfiles, and `<` / `>` resize it. Renaming offers to update every Link that points at the Note.

## Tasks

A Task is any `- [ ]` checkbox outside code blocks and `templates/`:

```markdown
- [ ] Call the bank due:fri pri:high #money
- [/] Draft the report due:+3d
- [x] Renew passport done:2026-10-01
- [-] Cancelled idea
```

`[ ]` and `[/]` are open, `[x]` is done, `[-]` is cancelled. When you leave insert mode, relative `due:` dates like `fri` or `+3d` are rewritten as ISO dates. Checking a Task off appends `done:<today>`, and unchecking removes it.

The Task List (`spc t`) groups open Tasks into Overdue, Today, Upcoming and No date. In it, `space` toggles, `enter` jumps to the line, `a` adds a Task to today's Daily Note, `D` shows all done and cancelled Tasks, and `/` filters by text, `#tag` or file.

## Configuration

Every key has a default, so no config file is needed. pholio never writes to these files.

**User config**, `$XDG_CONFIG_HOME/pholio/config.toml` (default `~/.config`, on macOS too):

| Key | Default | |
|---|---|---|
| `vault` | | Vault to open when no path is given |
| `theme` | `"default"` | `default`, `light`, `ansi16`, or a user theme |
| `mouse` | `true` | wheel scroll, clicks, sidebar drag-resize |
| `wrap` | `true` | soft word-wrap; `false` scrolls sideways |
| `conceal` | `true` | hide `[[ ]]`, `**`, backticks and link URLs off the cursor line |
| `open_daily_on_startup` | `true` | |

**Vault config**, `<vault>/.pholio/config.toml` (these keys can also go in the user config; the Vault file wins):

| Key | Default |
|---|---|
| `day_starts_at` | `"00:00"` (e.g. `"04:00"` keeps 1am on the previous day) |
| `daily_folder` | `"daily"` |
| `daily_template` | `"templates/daily.md"` |
| `zettel_folder` | `"zettel"` |
| `new_note_folder` | `""` (the Vault root) |
| `tasks_heading` | `"## Tasks"` (where the Task List's `a` adds Tasks) |

Daily templates can use `{{date}}`, `{{date:FMT}}`, `{{time}}`, `{{time:FMT}}`, `{{title}}`, `{{yesterday}}` and `{{tomorrow}}`, with moment-style formats such as `{{date:dddd, MMMM D}}`.

Unknown keys and bad values are reported once on the status line, and those keys use their defaults.

### Themes

Put themes in `~/.config/pholio/themes/<name>.toml` and select one with `theme = "<name>"` or `F7`. A theme only needs the colours and styles it changes; everything else falls back to the default theme. See [`internal/theme/themes/default.toml`](internal/theme/themes/default.toml) for every style slot.

```toml
[palette]
accent = "#ff8800"

[ui]
mode_normal = { fg = "bg", bg = "accent", bold = true }
```

## Working with other tools

pholio watches the Vault, so edits from Obsidian, Syncthing or `git pull` show up live. An unmodified buffer reloads silently. If you have unsaved changes, pholio warns you, and `:w` asks before overwriting. Syncthing `*.sync-conflict-*` files show in the sidebar but are left out of Tasks and Link resolution.

## Development

```sh
just          # list recipes
just test     # go test ./...
just ci       # vet, race tests and golangci-lint, as CI runs them
just golden   # regenerate View() snapshot files
```

The v1 behaviour is specified in [issue #17](https://github.com/tedkulp/pholio/issues/17), and domain terms (Note, Vault, Link, Task…) are defined in [`CONTEXT.md`](CONTEXT.md).
