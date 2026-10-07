# Changelog

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added
- `ctrl+shift+h` / `ctrl+shift+l` focus the sidebar / editor, like `ctrl+h` / `ctrl+l`, for multiplexers such as herdr that swallow those keys.
- Task Metadata uses Obsidian Tasks' `dataview` (`[due:: 2026-10-10]`) and `emoji` (`📅 2026-10-10`) formats, read on every Task line. Priority has Obsidian's five levels and sort order.
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
