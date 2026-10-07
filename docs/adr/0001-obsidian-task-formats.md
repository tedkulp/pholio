# Task Metadata uses Obsidian's Task Formats only

Task Metadata is written in the two formats of Obsidian's Tasks plugin, `dataview` (`[due:: 2026-10-10]`) and `emoji` (`📅 2026-10-10`), so a Vault reads the same in pholio and in Obsidian. pholio's own `due:`/`pri:`/`done:` format is dropped rather than kept as a third format, since it had no users yet and every extra format multiplies the parsing and write-back rules. `dataview` is the default because it is plain ASCII, which is easy to type in a terminal; emoji need a picker or digraphs.

## Consequences

- `due:2026-10-10` on a Task line is now plain text, not Task Metadata.
- Priority follows Obsidian's five levels, and a Task with no priority sorts between `medium` and `low`, as Obsidian sorts it.
