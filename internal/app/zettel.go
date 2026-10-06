package app

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"

	"github.com/tedkulp/pholio/internal/index"
	"github.com/tedkulp/pholio/internal/palette"
)

// zettelStamp is a Zettel's timestamp ID: YYYYMMDDHHmmss. Older Zettels
// have a 12-digit YYYYMMDDHHmm ID, which can never equal a new one.
const zettelStamp = "20060102150405"

// slug is title lowercased, with each run of characters that are not
// letters or digits turned into one "-", and "-" trimmed from both ends.
func slug(title string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(title) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			b.WriteRune(r)
			dash = false
			continue
		}
		dash = true
	}
	return b.String()
}

// zettelName is the file name of a Zettel created at t: "<stamp>-<slug>.md",
// or "<stamp>.md" when the title has no letters or digits.
func zettelName(t time.Time, title string) string {
	if s := slug(title); s != "" {
		return t.Format(zettelStamp) + "-" + s + ".md"
	}
	return t.Format(zettelStamp) + ".md"
}

// zettelRel is the Vault-relative path a Zettel titled title would get now.
// When another Zettel already uses the second's timestamp, a second is
// added until it is unique.
func (m Model) zettelRel(title string) (string, error) {
	folder := m.config().ZettelFolder
	taken := map[string]bool{}
	entries, err := m.deps.FS.ReadDir(m.abs(folder))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	for _, e := range entries {
		if stamp, ok := zettelStampOf(e.Name()); ok {
			taken[stamp] = true
		}
	}
	t := m.now()
	for taken[t.Format(zettelStamp)] {
		t = t.Add(time.Second)
	}
	return path.Join(folder, zettelName(t, title)), nil
}

// zettelStampOf is the timestamp ID a file name starts with, if any.
func zettelStampOf(name string) (string, bool) {
	n := len(zettelStamp)
	if len(name) < n+3 || !index.IsNote(name) {
		return "", false
	}
	if rest := index.TrimNoteExt(name[n:]); rest != "" && rest[0] != '-' {
		return "", false
	}
	stamp := name[:n]
	for _, r := range stamp {
		if r < '0' || r > '9' {
			return "", false
		}
	}
	return stamp, true
}

// linkName is the plain WikiLink name for a Vault path: its file name
// without .md.
func linkName(p string) string {
	return index.TrimNoteExt(filepath.Base(p))
}

// newZettel (spc z, :zettel) asks for a title, previewing the file name,
// then creates the Zettel. With an argument it skips the prompt.
func (m Model) newZettel(arg string) (Model, tea.Cmd) {
	if m.path() == "" {
		return m.say("A Zettel links back to its Origin: :w <name> first", true), nil
	}
	if strings.TrimSpace(arg) != "" {
		return m.createZettel(arg)
	}
	p := palette.New("New Zettel", palette.Type).
		WithPlaceholder("title").
		WithHint("enter create · esc close").
		WithInfo(func(q string) []string {
			if strings.TrimSpace(q) == "" {
				return nil
			}
			rel, err := m.zettelRel(strings.TrimSpace(q))
			if err != nil {
				return []string{err.Error()}
			}
			return []string{"→ " + rel}
		})
	return m.showPalette(p, func(m Model, ev palette.Event) (Model, tea.Cmd) {
		if ev.Kind != palette.Chosen || strings.TrimSpace(ev.Query) == "" {
			return m, nil
		}
		return m.createZettel(ev.Query)
	}), nil
}

// createZettel writes the Zettel with a back-Link to the Origin, puts a
// Link to it after the cursor in the Origin, saves the Origin and opens
// the Zettel.
func (m Model) createZettel(title string) (Model, tea.Cmd) {
	title = strings.TrimSpace(title)
	rel, err := m.zettelRel(title)
	if err != nil {
		return m.say("creating Zettel: "+err.Error(), true), nil
	}
	p := m.abs(rel)
	data := []byte(fmt.Sprintf("# %s\n\n[[%s]]\n", title, linkName(m.path())))
	if err := m.deps.FS.MkdirAll(filepath.Dir(p)); err != nil {
		return m.say("creating Zettel: "+err.Error(), true), nil
	}
	if err := m.deps.FS.WriteFile(p, data); err != nil {
		return m.say("creating Zettel: "+err.Error(), true), nil
	}
	if m.deps.Watch != nil {
		m.deps.Watch.Wrote(p, data)
	}
	if ix := m.index(); ix != nil {
		ix.Update(p, data)
	}
	m.ed.Engine().InsertAfterCursor("[[" + linkName(rel) + "]]")
	m, ok := m.save()
	if !ok {
		return m, nil
	}
	return m.openNote(p)
}
