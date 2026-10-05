# File watching and Vault scan cost in Go (as of 2026-10-05)

Answers GitHub issue [#11](https://github.com/tedkulp/pholio/issues/11) (part of map #1, feeds the External changes ticket #12). Every claim cites a primary source: library source at a pinned version from the Go module proxy (`go mod download`, 2026-10-05), the Linux kernel source and man pages, Apple's FSEvents guide, editor source code, or experiments run on this machine. Source references of the form `fsnotify@v1.10.1/backend_inotify.go:397` mean that file in the module at that version.

## TL;DR

- **fsnotify v1.10.1 is not recursive.** Recursive watching exists in the code but is switched off (`enableRecurse = false`, "Only enabled in tests for now"). pholio has to walk the Vault and `Add` every directory, then `Add` new directories as they show up.
- **New subdirectories race.** If a directory is created or moved in already holding files, those files produce **no events**. Both `mkdir -p a/b/c && echo > a/b/c/n.md` and `git switch` restoring `sub/s.md` lost the file in tests here. After adding a watch for a new directory, scan it.
- **inotify limits don't matter for a Vault.** pholio needs one watch per *directory*, not per file. The synthetic 50k-note Vault has 611 directories, against a kernel default of at least 8,192 watches (524,288 on Arch). Walking and adding them all took 23 ms.
- **macOS via fsnotify uses kqueue, which takes one fd per *file*.** A 50k-note Vault means about 50k open fds. Go raises the soft `RLIMIT_NOFILE` to `kern.maxfilesperproc` at startup, but this is the real scaling risk on macOS. Only `rjeczalik/notify` gives FSEvents (recursive, per-directory), and only with cgo.
- **Saves come in three shapes**, and the watcher must handle all of them as "path X may have changed": in-place truncate+write (nvim/vim by default, VS Code, Obsidian, git for modified files), rename-away+create (vim/nvim `backupcopy=auto` on the usual path), and temp+rename-over (Syncthing, `sed -i`). Debounce per path for about 100 ms, then re-read the file. Never trust the event type alone.
- **A full scan is cheap.** With a warm page cache on a Ryzen 7 9700X: 50k notes (156 MB, 376k Tasks, 939k Links) takes **~575 ms serial with regexes and ~220 ms in parallel**. A hand-written line parser takes **~157 ms serial and ~49 ms parallel**. 10k notes takes 27–136 ms; 1k notes takes 3–11 ms.
- **Parallelism helps 2–3x** once parsing costs more than reading. Choosing the parser matters about as much (regex is ~4x slower than byte scanning). A rescan-on-overflow strategy is affordable.

## 1. fsnotify on Linux (inotify)

### 1.1 What you get
- Current release **v1.10.1** (2026-05-04), Go 1.23+ (`go list -m -json github.com/fsnotify/fsnotify@latest`; `fsnotify@v1.10.1/CHANGELOG.md`).
- Backends: inotify (Linux), kqueue (BSD, macOS), ReadDirectoryChangesW (Windows), FEN (illumos). FSEvents, fanotify and polling are listed as "Not yet" / "Needs support in x/sys" (`README.md`, platform table; open issues [#11](https://github.com/fsnotify/fsnotify/issues/11), [#114](https://github.com/fsnotify/fsnotify/issues/114), [#9](https://github.com/fsnotify/fsnotify/issues/9)).
- **No recursion:** "Subdirectories are not watched (i.e. it's non-recursive)" (`fsnotify.go`, `Add` doc). The README FAQ says "you must add watches for any directory you want to watch (a recursive watcher is on the roadmap: #18)". Issue [#18](https://github.com/fsnotify/fsnotify/issues/18) "User-space recursive watcher" has been open since 2014. Its own proposed design is "Walk subdirectories to Add … skip hidden directories (.git, .hg)" plus "Listen for incoming Create events to watch additional directories".
- The `/...` recursive path syntax is implemented but gated: `var enableRecurse = false` with `if !enableRecurse { // Only enabled in tests for now.` (`fsnotify.go:502-510`). It is not public API, so don't rely on it.
- Events are `Create`, `Write`, `Remove`, `Rename` and `Chmod`. `Rename` carries the *old* name, and the new name arrives as a `Create`. The `RenamedFrom` field is unexported (`renamedFrom string`, `fsnotify.go`, `Event`), so pholio can't pair renames through the public API. "Close-write" (`IN_CLOSE_WRITE`) exists only as the unexported `xUnportableCloseWrite` (`fsnotify.go:231-251`), so "file finished writing" isn't available either. The library's own advice is to debounce: "wait a short time for more write events, resetting the wait period for every new event", with 100 ms in its example (`cmd/fsnotify/dedup.go`).
- The Events channel is unbuffered by default on inotify (`defaultBufferSize = 0`, `backend_inotify.go:145`). The kernel queue does the buffering.

### 1.2 Limits and overflow
- Every watched path is one inotify watch. The limits are `fs.inotify.max_user_watches` and `max_user_instances`, and hitting them gives "no space left on device" or "too many open files" (`fsnotify.go`, `Watcher` Linux notes; `README.md`).
- Kernel defaults (`fs/notify/inotify/inotify_user.c` at `e422777f`, `inotify_user_setup`): watches = 1% of RAM / `INOTIFY_WATCH_COST`, "limited to the range [8192, 1048576]". Instances = 128. `max_queued_events` = 16384. Distros raise these: Arch ships `max_user_watches = 524288`, `max_user_instances = 1024` (`/usr/lib/sysctl.d/10-arch.conf`, observed on this machine).
- The limit is **per user**, shared with every other watcher the user runs (editors, Syncthing, Dropbox, IDE language servers) (`inotify(7)`: "upper limit on the number of watches that can be created per real user ID").
- When the queue overflows, events are dropped and `IN_Q_OVERFLOW` is sent, which fsnotify surfaces as `ErrEventOverflow` on `Errors` (`inotify(7)` `/proc` interfaces; `fsnotify.go:262-270`; `backend_inotify.go:397`). The man page's guidance: "Robust applications should handle the possibility of lost events gracefully … it may be necessary to rebuild part or all of the application cache." **pholio's response to `ErrEventOverflow` should be a full rescan.** Section 4 shows that costs well under a second.
- Identical consecutive events are coalesced, "so an application can't use inotify to reliably count file events" (`inotify(7)`, NOTES).

### 1.3 New subdirectories: the race
`inotify(7)`: "If monitoring an entire directory subtree, and a new subdirectory is created in that tree or an existing directory is renamed into that tree, be aware that by the time you create a watch for the new subdirectory, new files (and subdirectories) may already exist inside the subdirectory. Therefore, you may want to scan the contents of the subdirectory immediately after adding the watch."

Reproduced with a fsnotify watcher that walks and `Add`s each new directory on `Create` (scratch program, fsnotify v1.10.1, tmpfs):

| Action | Events seen | Missed |
|---|---|---|
| `mkdir -p a/b/c && echo x > a/b/c/n.md` | `CREATE a` | `a/b`, `a/b/c`, `a/b/c/n.md` |
| `mv ../out/sub ./moved` (dir with `z.md`), then append to `moved/z.md` | `CREATE moved`, later `WRITE moved/z.md` | the pre-existing `moved/z.md` (seen only because it was written later) |
| `git switch` back to a branch that has `sub/s.md` | `CREATE sub` | `sub/s.md` |

So on a directory `Create`, the handler must (1) `Add` watches recursively and (2) **index every `.md` under it as if newly created**. Events that come in later for those files are harmless duplicates.

### 1.4 What to ignore
- `.git/` (git touches many files there on every operation), editor swap/backup files (`.note.md.swp`, `.swx`, `note.md~`, vim's `4913` probe file), Syncthing temp files (`.syncthing.*.tmp`), and `sed`'s `sedXXXXXX`. Filtering to `*.md` plus skipping dot-directories covers all of these (observed in §3).
- `Chmod`: "it's typically best to ignore Chmod events" (`README.md` FAQ). Vim and nvim emit `CHMOD` on every save (§3).
- On Linux, deleting a file that something still holds open emits `Chmod` and delays `Remove` until the last fd closes (`fsnotify.go`, Linux notes).

## 2. macOS

### 2.1 fsnotify = kqueue
- macOS uses the kqueue backend. "kqueue requires opening a file descriptor for every file that's being watched; so if you're watching a directory with five files then that's six file descriptors" (`fsnotify.go`, kqueue notes). The code confirms that `watchDirectoryFiles` opens a watch on each directory entry (`backend_kqueue.go:582-617`), using `O_EVTONLY` (`system_darwin.go:8`).
- New files are found by re-reading the directory on `NOTE_WRITE` and diffing against a "seen" set (`dirChange`/`sendCreateIfNew`, `backend_kqueue.go:622-667`). So every create in a large directory costs an `os.ReadDir` of that whole directory.
- On kqueue, a `Write` on a *directory* means its contents changed. This doesn't happen on inotify (`fsnotify.go`, `Events` doc and `Write` op doc).
- Go raises its own soft `RLIMIT_NOFILE` to the hard limit at startup, capped on darwin at `kern.maxfilesperproc` (`$GOROOT/src/syscall/rlimit.go` `init`, `rlimit_darwin.go` `adjustFileLimit`; go1.27.1). For pholio, the fd count is roughly the number of Notes plus the number of directories. That makes a 50k-note Vault **≈51k fds** on macOS, compared with ~611 inotify watches on Linux. Whether that fits depends on the Mac's `kern.maxfilesperproc`, which wasn't measured (no Mac available). Treat a large Vault on macOS as the scaling risk.

### 2.2 FSEvents (the native recursive API)
- FSEvents is recursive and directory-granular by design. The guide tells clients: "For each event, you should scan the directory at the specified path." It coalesces, and sets `kFSEventStreamEventFlagMustScanSubDirs` when "you must recursively rescan the path listed in the event", including when events were dropped. It says "treat the events list as advisory", and "you must start monitoring the directory *before* you start scanning it" ([Apple, File System Events Programming Guide: Using the FSEvents Framework](https://developer.apple.com/library/archive/documentation/Darwin/Conceptual/FSEvents_ProgGuide/UsingtheFSEventsFramework/UsingtheFSEventsFramework.html)).
- Go options:
  - `github.com/rjeczalik/notify` v0.9.3 (2023-01-12; repo still pushed 2026-06; 937 stars): FSEvents on darwin with recursive `dir/...` watches, but **only with cgo**. The FSEvents file is `//go:build darwin && !kqueue && cgo`, and without cgo it falls back to kqueue (`watcher_kqueue.go` build tag `(darwin && kqueue) || (darwin && !cgo)`). It requests per-file events with `kFSEventStreamCreateFlagFileEvents | kFSEventStreamCreateFlagNoDefer` (`watcher_fsevents_cgo.go:38`). On Linux it is still inotify with a user-space recursive tree, so it has the same §1.3 race.
  - `github.com/fsnotify/fsevents` v0.2.0 (last push 2024-05; cgo; darwin-only).
- Conclusion: the portable design is the same on both OSes. Treat any event as a hint to (re)scan that path or directory, and keep a full rescan as the fallback. That design also makes swapping the macOS backend to FSEvents later a contained change.

## 3. How real editors and tools write files

Measured with a fsnotify v1.10.1 recursive logger on Linux 7.2 (tmpfs), editing `note.md`. `-u NONE` / `--clean` means stock defaults. **Gotcha:** Vim's `backupskip` default contains `/tmp/*`, so tests under `/tmp` silently skip the backup step and look like in-place writes. The rows below set `backupskip=` to match a normal Vault path.

| Writer | Event sequence on `note.md` (ignoring swap files) | Shape |
|---|---|---|
| vim 9.2, defaults (`backupcopy=auto`, `writebackup`) | probe `4913` create/remove → `RENAME note.md` → `CREATE note.md~` → `CREATE note.md` → `WRITE` → `CHMOD` → `REMOVE note.md~` | rename-away + create |
| nvim 0.12.5, defaults (`backupdir=.,…`) | same as vim | rename-away + create |
| vim/nvim `backupcopy=no` | `RENAME note.md` → `CREATE note.md~` → `CREATE note.md` → `WRITE` | rename-away + create |
| vim `backupcopy=yes`, or `auto` when the file has a hard link | `CREATE note.md~` … `WRITE note.md` → `CHMOD` → `REMOVE note.md~` | in place |
| vim/nvim with backup skipped (file under `/tmp`, or `backupdir` on another filesystem) | `WRITE note.md` → `CHMOD` | in place |
| VS Code (desktop, Linux/macOS) | `open(path, 'w')` = truncate + write. Atomic temp+rename exists in the provider (`opts.atomic.postfix`) but text-file saves don't request it; it's used for state and user-data files | in place |
| Obsidian 1.13.7 desktop | `FileSystemAdapter.write` → `fsPromises.writeFile(path, data, "utf8")` = truncate + write | in place |
| Syncthing | "never writes directly to a destination file … changes are made to a temporary copy which is then moved in place", named `.syncthing.<name>.tmp`. Also creates `<name>.sync-conflict-<date>-<time>-<id>.md` on conflicts | temp + rename-over |
| `sed -i` | `CREATE sedXXXX` → `WRITE` → `RENAME sedXXXX` → `CREATE note.md` | temp + rename-over |
| `git switch` (file changed between branches) | `REMOVE note.md` → `CREATE note.md` → `WRITE` | unlink + create |
| `git switch` (dir removed / restored) | `REMOVE sub/s.md`, `REMOVE sub` / `CREATE sub` only (file missed, §1.3) | dir create race |

Sources: experiments above. Vim/nvim option values from `:set writebackup? backupcopy? backupdir?` under `-u NONE` / `--clean`. VS Code `src/vs/platform/files/node/diskFileSystemProvider.ts` at `d524f802` (`writeFile` lines ~250 and ~444, plus `atomic` usage found by code search only in `stateService.ts`, `fileUserDataProvider.ts` and extension management). Obsidian from the minified `/usr/lib/obsidian/obsidian.asar` bundle (1.13.7, closed source, so this is a reading of shipped JS, not documented behaviour). Syncthing [docs `users/syncing.rst` "Temporary Files" and "Conflicting Changes"](https://github.com/syncthing/docs/blob/a86f6a1c2c6bf73e2817e3f045d292d9f26b840b/users/syncing.rst).

Implications for pholio:
- In the rename-away and unlink shapes, `note.md` is briefly **absent**. A watcher that reacts instantly to `Rename`/`Remove` would drop the Note's Tasks and Links and then re-add them. **Debounce per path (≈100 ms), then `stat` + re-read.** If the file exists, it was modified. If not, it was removed.
- Because pholio watches directories, rename-over is seen as a `Create` of the final name. Don't treat `Create` of a known path as "new Note". Treat it as "changed".
- Syncthing conflict copies are real `.md` files, so their Tasks will show up twice in the Task List. That's a product decision for #12, not a watcher bug.
- pholio's own saves will echo back as events. Record (path, size, mtime or hash) at write time and suppress the matching event.

## 4. Vault scan cost (measured)

**Setup:** throwaway Go program (not committed). It generates synthetic Vaults: 1/5 of the notes under `daily/YYYY/MM/`, the rest under `zettel/dNNN/` with 200 per folder. Each note is ~3.1 KB with a title, 4–11 paragraphs each holding 2 `[[links]]` (one with an alias), and on about half the paragraphs a pair of Tasks, one with `due:` / `pri:` / `#tag` metadata and one with a Link. One run = `filepath.WalkDir` collecting `*.md` paths, then `os.ReadFile` + parse each, counting Tasks and Links. Two parsers:
- **regex:** `bufio.Scanner` lines with a `bytes.Contains` prefilter, then `^\s*[-*+] \[([ xX/\-])\] (.*)$` and a `[[target#heading|alias]]` regex.
- **hand:** `bytes.IndexByte` line split plus a fixed-position checkbox test and `[[`…`]]` scanning.

Both produce identical counts. Parallel = one walker feeding N goroutines over a channel. Each number is the median of 7 runs.

**Machine caveats:** AMD Ryzen 7 9700X (8C/16T), 30 GB RAM, Linux 7.2.9 (CachyOS), go1.27.1. Vaults were on **tmpfs**, so every run is page-cache-warm. **Disk-cold numbers were not measured** (dropping the page cache needs root). A real cold start on NVMe adds per-file read latency on top of the "read" column, and on spinning or network disks, much more. Synthetic notes are uniform; real Vaults have a long tail of big files.

| Notes | Dirs | Data | Tasks | Links | walk only | walk+read (serial) | regex serial | regex 4 workers | regex 16 workers | hand serial | hand 4 workers | hand 16 workers |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 1,000 | 16 | 3.1 MB | 7,394 | 18,703 | 0.2 ms | 2.4 ms | 10.7 ms | 5.6 ms | 5.8 ms | 3.1 ms | 1.3 ms | 1.2 ms |
| 10,000 | 126 | 30.8 MB | 74,288 | 186,784 | 2 ms | 22 ms | 136 ms | 55 ms | 52 ms | 27 ms | 11 ms | 10 ms |
| 50,000 | 611 | 155.5 MB | 375,766 | 938,557 | 18 ms | 125 ms | 575 ms | 244 ms | 220 ms | 157 ms | 62 ms | 49 ms |

Watcher setup (walk + `fsnotify.Add` per directory, Linux): 0.5 ms / 4.7 ms / 23 ms for the 1k / 10k / 50k Vaults.

Reading the table:
- Warm reads cost ~2.5 µs per file. Regex parsing costs ~11 µs per file and dominates the serial time. A hand-written scanner brings parsing close to the cost of reading.
- Parallelism gives 2–2.6x, and going from 4 to 16 workers adds little. The single walker and syscall overhead become the limit. 4 workers capture most of the gain.
- Even the worst measured case (50k notes, regex, serial) is about 0.6 s warm. A full rescan after `ErrEventOverflow` or `MustScanSubDirs` is affordable. A startup scan of a 10k-note Vault fits easily inside the first frame or two in the background. Persisting an index to skip the startup scan isn't justified by these numbers, though a disk-cold 50k Vault is the case to re-measure if it ever matters.

## 5. Recommendation for #12 (External changes)

1. Use `github.com/fsnotify/fsnotify` v1.10.1, pure Go and no cgo. Walk the Vault at startup, skip dot-directories, and `Add` each directory.
2. On a directory `Create`, `Add` its subtree recursively **and** index every `.md` in it (§1.3).
3. Debounce events per path for about 100 ms. Then `stat`: if the file exists, re-parse it; if not, remove it from the index. Ignore `Chmod` and non-`.md` names. Suppress echoes of pholio's own writes.
4. On `ErrEventOverflow` (or any watcher error), run a full rescan, in parallel with about 4 workers.
5. On macOS, accept kqueue's fd-per-file cost for now. Keep the watcher behind a small interface so an FSEvents backend (`rjeczalik/notify` with cgo) can replace it if large Vaults hit fd limits.
6. Use a hand-written line parser for Tasks and Links rather than regexes. It is ~4x faster and the grammar is simple.
