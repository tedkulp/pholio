# Changelog

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

## [0.7.0] - 2026-10-20

### Added
- Yank, delete and put use the system clipboard, like vim's `clipboard=unnamedplus`: every write to the unnamed register also goes to the clipboard, and `p`/`P` paste what's on it, so text moves between pholio and other apps. `"+` and `"*` name the clipboard; `"a`–`"z` and `"0`–`"9` stay internal. Text copied in another app pastes as lines when it ends in a newline, else inline; pholio's own yanks keep their shape (lines or a block). pholio uses `wl-copy`/`wl-paste`, `xclip` or `xsel`, or `pbcopy`/`pbpaste`, and without them (as over SSH) writes the clipboard with OSC 52, where puts use the internal register. In a terminal multiplexer pane with no `WAYLAND_DISPLAY` or `DISPLAY` (a herdr or tmux server started outside the desktop), pholio uses the Wayland display if exactly one live `wayland-*` socket is in `$XDG_RUNTIME_DIR`.

## [0.6.1] - 2026-10-12

### Added
- `meetings_subfolder` puts new Meeting Notes in date-based folders under `meetings_folder`, such as `"YYYY/MM"`, by the Meeting's start date, like `daily_subfolder` does for Daily Notes. File names and Daily Note back-links are unchanged; a name already used anywhere under `meetings_folder` counts up to `-2`. Existing Meeting Notes stay where they are.

## [0.6.0] - 2026-10-12

### Added
- Meeting Notes. `[[calendars]]` in the user config lists ICS calendar feeds, and `spc m` (`:meetings`) opens the Meeting picker on Today's Meetings from all of them: `h` / `l` change day, `t` returns to Today, `r` refreshes. `enter` opens the Meeting's Meeting Note, or creates it in `meetings_folder` from `meeting_template` and links it under `meetings_heading` in that day's Daily Note. Recurring and moved Meetings and real-world timezones are handled, and feeds are cached so the picker opens at once and works offline.

## [0.5.0] - 2026-10-08

### Added
- `>` and `<` indent and outdent lines by one tab: `>>`, `<<`, with a count or a motion (`>j`, `>ip`), and on a visual selection.
- `.` repeats a change made from visual mode (`d`, `c`, `>`, `<`, `~`, `J`) on a selection of the same size, starting at the cursor.
- Blockwise visual mode: `ctrl+v` selects a block of columns, measured in display cells. `d`, `y`, `c`, `>`, `<`, `~`, `J`, `I` and `A` act on it, `$` extends it to the end of every line, and a yanked block pastes back as a block with `p`/`P`.
- Fenced code blocks are syntax-coloured by the language on the opening fence (```` ```go ````, ```` ~~~ python ````), through the new `[code]` theme slots. A block with no language, or one pholio doesn't know, stays in `markdown.code`.

## [0.4.0] - 2026-10-07

### Added
- `ctrl+shift+h` / `ctrl+shift+l` focus the sidebar / editor, like `ctrl+h` / `ctrl+l`, for multiplexers such as herdr that swallow those keys.
- Task Metadata uses Obsidian Tasks' `dataview` (`[due:: 2026-10-10]`) and `emoji` (`📅 2026-10-10`) formats, read on every Task line. Priority has Obsidian's five levels and sort order.
- The Task Editor, a pop-up form for one Task's description, Status, due, scheduled and start dates, and priority. Open it with `spc T` or `:task` on a Task's line, or `e` in the Task List. The Task List's `a` now opens it empty to add a Task.
- The help popup (`spc ?`) lists the ex commands too.
- `task_format` (`"dataview"` or `"emoji"`) picks the format pholio writes done dates in when a Task has no metadata yet.

### Changed
- pholio's own `due:`, `pri:` and `done:` words are no longer read as Task Metadata; they are plain text. Rewrite them as `[due:: …]`, `[priority:: …]` and `[completion:: …]`.

## [0.3.0] - 2026-10-06

### Added
- `pholio init [folder]` creates a new Vault: its Daily Note and Zettel folders, a starter daily Template and a commented `.pholio/config.toml`, and sets `vault` in the user config if it is unset.
- `~~text~~` is drawn struck through, in the new `markdown.strike` theme style, and `conceal` hides its `~~` off the cursor line.

## [0.2.0] - 2026-10-06

### Added
- `daily_subfolder` puts new Daily Notes in date-based folders, such as `daily/2026/10/2026-10-06.md`.
- `spc ?` opens a filterable help popup with every key. `enter` on a row runs it.
- `spc q` quits, asking first about unsaved changes, like `ctrl+q`.

### Changed
- A new Zettel's timestamp ID has seconds: `YYYYMMDDHHmmss`. Existing 12-digit Zettels are left as they are.
- An empty checkbox, such as `- [ ]` with nothing after it, is no longer a Task and stays out of the Task List.

## [0.1.0] - 2026-10-06

### Added
- A Vault is a plain folder of `.md` files, opened with a file tree sidebar and a vim-style modal editor.
- Daily Notes, with templates.
- A Vault-wide Task List with `due:` and `done:` dates.
- `[[wikilinks]]` with link navigation and Backlinks.
- Zettel creation from inside any Note, linked back to its Origin.
- File operations: create, rename or move (rewriting Links), and delete to the Trash.
- Fuzzy find and Vault-wide grep.
- Themes and a user/Vault config split.
- External changes picked up by a file watcher.
- `pholio --version`.
- Release builds: linux and darwin archives, deb and rpm packages, and a Homebrew cask.
