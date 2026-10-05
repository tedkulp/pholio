package app

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/tedkulp/pholio/internal/index"
	"github.com/tedkulp/pholio/internal/palette"
	"github.com/tedkulp/pholio/internal/relink"
)

// Vault file operations: new (spc n, :new), add (a in the tree), rename and
// move (r in the tree, :rename) with Link rewriting, and delete (d in the
// tree, :delete) to the Trash. None of them can be undone.

// newNote (spc n, :new <name>) opens an unsaved buffer for a new Note. A
// bare name goes in new_note_folder; a name with folders is Vault-relative.
func (m Model) newNote(arg string) (Model, tea.Cmd) {
	if strings.TrimSpace(arg) != "" {
		return m.openNew(arg)
	}
	p := palette.New("New Note", palette.Type).
		WithPlaceholder("name, or folder/name").
		WithHint("enter open · esc close").
		WithInfo(func(q string) []string {
			if strings.TrimSpace(q) == "" {
				return nil
			}
			rel, err := m.newNotePath(q)
			if err != nil {
				return []string{err.Error()}
			}
			return []string{rel}
		})
	return m.showPalette(p, func(m Model, ev palette.Event) (Model, tea.Cmd) {
		if ev.Kind != palette.Chosen || strings.TrimSpace(ev.Query) == "" {
			return m, nil
		}
		return m.openNew(ev.Query)
	}), nil
}

// newNotePath is the Vault-relative path :new gives name.
func (m Model) newNotePath(name string) (string, error) {
	name = strings.TrimSpace(name)
	rel := strings.TrimPrefix(name, "/")
	if !strings.Contains(name, "/") && m.session != nil {
		rel = path.Join(m.session.Config.NewNoteFolder, rel)
	}
	return vaultPath(rel, true)
}

// vaultPath cleans a typed Vault-relative path, adding ".md" when note is
// set and it has none.
func vaultPath(typed string, note bool) (string, error) {
	rel := path.Clean(strings.TrimPrefix(strings.TrimSpace(typed), "/"))
	if rel == "." || !inVault(rel) {
		return "", fmt.Errorf("not a path in the Vault: %q", typed)
	}
	if note && !isNote(rel) {
		rel += ".md"
	}
	return rel, nil
}

func isNote(rel string) bool { return strings.HasSuffix(strings.ToLower(rel), ".md") }

// openNew opens the Note :new names: as it is when it exists, else as an
// unsaved buffer.
func (m Model) openNew(name string) (Model, tea.Cmd) {
	rel, err := m.newNotePath(name)
	if err != nil {
		return m.say(err.Error(), true), nil
	}
	p := m.abs(rel)
	if _, err := m.deps.FS.Stat(p); err == nil {
		return m.openNote(p)
	}
	return m.openAt(p, func(m Model) Model {
		return m.say("New Note "+rel+": :w to create it", false)
	})
}

// vaultRel is the Vault-relative slash path of the OS path p ("." for the
// Vault itself).
func (m Model) vaultRel(p string) string {
	r, err := filepath.Rel(m.vault, p)
	if err != nil {
		return p
	}
	return filepath.ToSlash(r)
}

// treeAdd (a in the tree) asks for a name, prefilled with the selected
// folder, and creates that Note, or that folder when it ends in "/".
func (m Model) treeAdd() (Model, tea.Cmd) {
	prefill := ""
	if n, ok := m.side.Selected(); ok {
		dir := n.Path
		if !n.Dir {
			dir = filepath.Dir(dir)
		}
		if rel := m.vaultRel(dir); rel != "." {
			prefill = rel + "/"
		}
	}
	p := palette.New("Add", palette.Type).
		WithPlaceholder("name, or folder/ for a folder").
		WithHint("enter create · esc close").
		WithQuery(prefill)
	return m.showPalette(p, func(m Model, ev palette.Event) (Model, tea.Cmd) {
		if ev.Kind != palette.Chosen {
			return m, nil
		}
		return m.add(ev.Query)
	}), nil
}

// add creates the Note or, with a trailing "/", the folder named by the
// Vault-relative path typed, and opens or reveals it.
func (m Model) add(typed string) (Model, tea.Cmd) {
	folder := strings.HasSuffix(strings.TrimSpace(typed), "/")
	rel, err := vaultPath(typed, !folder)
	if err != nil {
		return m.say(err.Error(), true), nil
	}
	p := m.abs(rel)
	if folder {
		if err := m.deps.FS.MkdirAll(p); err != nil {
			return m.say(err.Error(), true), nil
		}
		m.side = m.side.Refresh().Reveal(p)
		return m.say("Created "+rel+"/", false), nil
	}
	if _, err := m.deps.FS.Stat(p); err == nil {
		return m.openNote(p)
	}
	if err := m.file.WriteFile(p, nil); err != nil {
		return m.say(err.Error(), true), nil
	}
	m.side = m.side.Refresh()
	return m.openAt(p, func(m Model) Model { return m.say("Created "+rel, false) })
}

// renameNote (:rename [path]) renames or moves the open Note.
func (m Model) renameNote(arg string) (Model, tea.Cmd) {
	if m.path() == "" {
		return m.say("E32: No file name", true), nil
	}
	if _, err := m.deps.FS.Stat(m.path()); err != nil {
		return m.say(m.rel(m.path())+" is not on disk yet: :w first", true), nil
	}
	if strings.TrimSpace(arg) != "" {
		return m.move(m.path(), arg)
	}
	return m.askRename(m.path())
}

// treeRename (r in the tree) renames or moves the selected file or folder.
func (m Model) treeRename() (Model, tea.Cmd) {
	n, ok := m.side.Selected()
	if !ok {
		return m, nil
	}
	return m.askRename(n.Path)
}

// askRename asks for the new path of p, prefilled with its current one.
func (m Model) askRename(p string) (Model, tea.Cmd) {
	pal := palette.New("Rename or move", palette.Type).
		WithHint("enter rename · esc close").
		WithQuery(m.vaultRel(p))
	return m.showPalette(pal, func(m Model, ev palette.Event) (Model, tea.Cmd) {
		if ev.Kind != palette.Chosen || strings.TrimSpace(ev.Query) == "" {
			return m, nil
		}
		return m.move(p, ev.Query)
	}), nil
}

// move renames the file or folder at from to the Vault-relative path typed.
// When that changes the text any Link needs, it asks first whether to
// rewrite them.
func (m Model) move(from, typed string) (Model, tea.Cmd) {
	info, err := m.deps.FS.Stat(from)
	if err != nil {
		return m.say(err.Error(), true), nil
	}
	fromRel := m.vaultRel(from)
	toRel, err := vaultPath(typed, !info.IsDir() && isNote(fromRel))
	if err != nil {
		return m.say(err.Error(), true), nil
	}
	switch {
	case toRel == fromRel:
		return m, nil
	case info.IsDir() && strings.HasPrefix(toRel+"/", fromRel+"/"):
		return m.say("Can't move "+fromRel+"/ into itself", true), nil
	}
	to := m.abs(toRel)
	if _, err := m.deps.FS.Stat(to); err == nil {
		return m.say(toRel+" already exists", true), nil
	}

	ix := m.index()
	if ix == nil || (!info.IsDir() && !isNote(fromRel)) {
		return m.moveFiles(from, to, nil, nil), nil
	}
	if !m.indexReady() {
		return m.say("indexing… try again in a moment", false), nil
	}
	m = m.syncIndex()
	notes := ix.Notes()
	moves := noteMoves(notes, fromRel, toRel, info.IsDir())
	edits := relink.Plan(notes, moves)
	if len(edits) == 0 {
		return m.moveFiles(from, to, notes, moves), nil
	}
	links := 0
	for _, e := range edits {
		links += e.Links
	}
	rewrite := func(m Model) (Model, tea.Cmd) {
		m = m.moveFiles(from, to, notes, moves)
		if m.problem {
			return m, nil
		}
		return m.rewrite(edits, links), nil
	}
	keep := func(m Model) (Model, tea.Cmd) { return m.moveFiles(from, to, notes, moves), nil }
	q := fmt.Sprintf("Update %s in %s? [Y/n]", plural(links, "Link"), plural(len(edits), "Note"))
	return m.ask(q, map[string]func(Model) (Model, tea.Cmd){
		"y": rewrite, "Y": rewrite, "enter": rewrite, "n": keep, "N": keep,
	}), nil
}

// noteMoves lists the Notes a move takes along, Vault-relative before →
// after.
func noteMoves(notes []index.Note, fromRel, toRel string, dir bool) map[string]string {
	if !dir {
		return map[string]string{fromRel: toRel}
	}
	moves := map[string]string{}
	for _, n := range notes {
		if rest, ok := strings.CutPrefix(n.Path, fromRel+"/"); ok {
			moves[n.Path] = toRel + "/" + rest
		}
	}
	return moves
}

// moveFiles renames from to to on disk and follows it in the index, the
// open buffer, the jumplist and the tree. notes are the index's Notes
// before the move.
func (m Model) moveFiles(from, to string, notes []index.Note, moves map[string]string) Model {
	if err := m.deps.FS.MkdirAll(filepath.Dir(to)); err != nil {
		return m.say(err.Error(), true)
	}
	if err := m.deps.FS.Rename(from, to); err != nil {
		return m.say(err.Error(), true)
	}
	if ix := m.index(); ix != nil {
		for _, n := range notes {
			if dst, ok := moves[n.Path]; ok {
				ix.Remove(m.abs(n.Path))
				ix.Update(m.abs(dst), []byte(n.Contents))
			}
		}
	}
	if p, ok := movedPath(m.path(), from, to); ok {
		e := m.ed.Engine()
		m.file.path, e.Path = p, p
		m.ed = m.newEditor(e)
	}
	m.jumps = m.jumps.moved(from, to)
	m.side = m.side.Refresh().Reveal(to)
	return m.say("Renamed "+m.vaultRel(from)+" → "+m.vaultRel(to), false)
}

// movedPath is where p is after from moved to to; ok is false when the
// move does not take p along.
func movedPath(p, from, to string) (string, bool) {
	if p == from {
		return to, true
	}
	if rest, ok := strings.CutPrefix(p, from+string(filepath.Separator)); ok {
		return filepath.Join(to, rest), true
	}
	return "", false
}

// moved follows a rename in the jumplist's entries.
func (j jumplist) moved(from, to string) jumplist {
	list := slices.Clone(j.list)
	for i, e := range list {
		if p, ok := movedPath(e.path, from, to); ok {
			list[i].path = p
		}
	}
	return jumplist{list: list, i: j.i}
}

// rewrite applies Link edits after a move: into the open buffer when it is
// dirty, to disk otherwise.
func (m Model) rewrite(edits []relink.Edit, links int) Model {
	var failed []string
	for _, ed := range edits {
		p := m.abs(ed.To)
		if p == m.path() {
			e := m.ed.Engine()
			if e.Dirty {
				e.Reload(ed.Contents)
				e.Dirty = true
				m = m.syncIndex()
				continue
			}
		}
		if err := m.file.WriteFile(p, []byte(ed.Contents)); err != nil {
			failed = append(failed, ed.To+": "+err.Error())
			continue
		}
		if p == m.path() {
			m.ed.Engine().Reload(ed.Contents)
			m.fed = m.ed.Engine().Buf.Version()
		}
	}
	if len(failed) > 0 {
		return m.say("Updating Links failed: "+strings.Join(failed, "; "), true)
	}
	return m.say(m.message+fmt.Sprintf(" · updated %s in %s", plural(links, "Link"), plural(len(edits), "Note")), false)
}

// deleteNote (:delete) deletes the open Note.
func (m Model) deleteNote() (Model, tea.Cmd) {
	if m.path() == "" {
		return m.say("E32: No file name", true), nil
	}
	return m.confirmDelete(m.path())
}

// treeDelete (d in the tree) deletes the selected file or folder.
func (m Model) treeDelete() (Model, tea.Cmd) {
	n, ok := m.side.Selected()
	if !ok {
		return m, nil
	}
	return m.confirmDelete(n.Path)
}

// confirmDelete asks before moving p to the Trash, with how many Links
// point at a Note or how many files a folder holds. Only y deletes.
func (m Model) confirmDelete(p string) (Model, tea.Cmd) {
	info, err := m.deps.FS.Stat(p)
	if errors.Is(err, fs.ErrNotExist) {
		return m.say(m.vaultRel(p)+" is not on disk", true), nil
	}
	if err != nil {
		return m.say(err.Error(), true), nil
	}
	rel := m.vaultRel(p)
	q := fmt.Sprintf("Delete %s? y/N", rel)
	switch {
	case info.IsDir():
		q = fmt.Sprintf("Delete %s/ (%s)? y/N", rel, plural(m.countFiles(p), "file"))
	case isNote(rel) && m.indexReady():
		m = m.syncIndex()
		q = fmt.Sprintf("Delete %s (%s)? y/N", rel, plural(len(m.index().Backlinks(rel)), "Backlink"))
	}
	no := func(m Model) (Model, tea.Cmd) { return m, nil }
	return m.ask(q, map[string]func(Model) (Model, tea.Cmd){
		"y":     func(m Model) (Model, tea.Cmd) { return m.trash(p, info.IsDir()), nil },
		"n":     no,
		"N":     no,
		"enter": no,
	}), nil
}

// countFiles counts the files under the folder dir.
func (m Model) countFiles(dir string) int {
	entries, err := m.deps.FS.ReadDir(dir)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() {
			n += m.countFiles(filepath.Join(dir, e.Name()))
		} else {
			n++
		}
	}
	return n
}

// trash moves p to the Trash, drops it from the index and closes the open
// buffer when it was in it.
func (m Model) trash(p string, dir bool) Model {
	if m.deps.Trash == nil {
		return m.say("No trash available: nothing deleted", true)
	}
	if err := m.deps.Trash.Trash(p); err != nil {
		return m.say("Deleting "+m.vaultRel(p)+": "+err.Error(), true)
	}
	if ix := m.index(); ix != nil {
		ix.Remove(p)
		if dir {
			for _, n := range ix.Notes() {
				if _, ok := movedPath(m.abs(n.Path), p, p); ok {
					ix.Remove(m.abs(n.Path))
				}
			}
		}
	}
	if _, ok := movedPath(m.path(), p, p); ok {
		f := m.focus
		m, _ = m.switchTo("")
		m.focus = f
	}
	m.side = m.side.Refresh()
	return m.say("Moved "+m.vaultRel(p)+" to the trash", false)
}

// plural is "1 Link", "3 Links".
func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
