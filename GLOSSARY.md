# pholio

A terminal app for keeping a personal markdown knowledge base, organized around daily notes, Zettelkasten-style linked notes, and one vault-wide TODO list.

## Language

### Vault and notes

**Vault**:
The root folder holding every note pholio manages. It is a plain directory of markdown files.
_Avoid_: workspace, library, repo

**Note**:
One markdown file inside the Vault.
_Avoid_: page, document, entry

**Daily Note**:
The Note for one calendar day, created from the daily Template the first time that day is opened.
_Avoid_: journal, diary entry

**Today**:
The current day as pholio counts it. It starts at the configured day-start hour, not necessarily at midnight, so 1am can still be yesterday. Every "today" (the Daily Note, Overdue, `done:` stamps) means this day.
_Avoid_: current date, now

**Zettel**:
A Note on a single idea, created on the spot from inside another Note. Its name carries a timestamp ID so it stays unique and stable when its title changes.
_Avoid_: card, atomic note, permanent note

**Template**:
A Note whose contents become the starting text of a new Note, after its placeholders are filled in.
_Avoid_: skeleton, boilerplate

### Connections

**Link**:
A `[[wikilink]]` from one Note to another.
_Avoid_: reference, ref

**Backlink**:
A Link in the reverse direction, seen from the Note being linked to.
_Avoid_: inbound link, mention

**Origin**:
The Note (usually a Daily Note) that was open when a Zettel was created. The Zettel links back to it.
_Avoid_: parent, source

### Tasks

**Task**:
One markdown checkbox line (`- [ ]` / `- [x]`) anywhere in the Vault, along with its inline metadata. An empty checkbox, with nothing but whitespace after it, is not a Task.
_Avoid_: TODO item, todo, action item

**Task Metadata**:
Dates, a priority and `#tags` written inline on a Task's line, in a Task Format, such as `[due:: 2026-10-10] [priority:: high] #work` or `📅 2026-10-10 ⏫ #work`.
_Avoid_: attributes, properties

**Task Format**:
The syntax a Task's Metadata is written in: `dataview` (`[due:: 2026-10-10]`) or `emoji` (`📅 2026-10-10`), the two formats of Obsidian's Tasks plugin. pholio reads both and writes new Metadata in the Vault's default format, unless the Task already uses the other one.
_Avoid_: metadata style, task syntax

**Task Status**:
The state shown in a Task's checkbox: open `[ ]`, in progress `[/]` (still open), done `[x]`, or cancelled `[-]`. A done Task records the day it was finished as its done date (`[completion:: YYYY-MM-DD]` or `✅ YYYY-MM-DD`).
_Avoid_: state, completed (use "done")

**Task List**:
The single view that gathers every Task in the Vault.
_Avoid_: master list, agenda, inbox
