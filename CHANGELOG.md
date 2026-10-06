# Changelog

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added
- `daily_subfolder` puts new Daily Notes in date-based folders, such as `daily/2026/10/2026-10-06.md`.
- Wait a moment after `spc` and a which-key popup lists every Leader key above the status line.
- `spc ?` opens a filterable help popup with every key. `enter` on a row runs it.

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
