package daily

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tedkulp/pholio/internal/dates"
	"github.com/tedkulp/pholio/internal/seam"
)

// Notes are the Daily Notes of one Vault.
type Notes struct {
	FS       seam.FS
	Vault    string // absolute OS path
	Folder   string // daily_folder, Vault-relative and slash-separated
	Template string // daily_template, Vault-relative and slash-separated
	// Subfolder is daily_subfolder: a dates.Format layout, such as
	// "YYYY/MM", for the folders under Folder that new Daily Notes go in.
	// "" puts them directly in Folder.
	Subfolder string
}

func (n Notes) dir() string { return filepath.Join(n.Vault, filepath.FromSlash(n.Folder)) }

// Path is where day's Daily Note lives, in the configured layout.
func (n Notes) Path(day time.Time) string {
	dir := n.dir()
	if n.Subfolder != "" {
		dir = filepath.Join(dir, filepath.FromSlash(dates.Format(day, n.Subfolder)))
	}
	return filepath.Join(dir, day.Format(dates.ISO)+".md")
}

// Day reports the day of the Daily Note at path, or false when path is not
// a Daily Note: a YYYY-MM-DD.md file at any depth under the daily folder,
// whatever the Subfolder layout.
func (n Notes) Day(path string) (time.Time, bool) {
	rel, err := filepath.Rel(n.dir(), filepath.Clean(path))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return time.Time{}, false
	}
	return parseName(filepath.Base(path))
}

func parseName(name string) (time.Time, bool) {
	stem, ok := strings.CutSuffix(name, ".md")
	if !ok {
		return time.Time{}, false
	}
	d, err := time.ParseInLocation(dates.ISO, stem, time.Local)
	return d, err == nil
}

// Entry is one existing Daily Note: its day and where it lives.
type Entry struct {
	Day  time.Time
	Path string
}

// days lists the Daily Notes anywhere under the daily folder, oldest
// first, one per day. When a day has two, the one at Path(day) wins. A
// missing daily folder has none.
func (n Notes) days() ([]Entry, error) {
	found := map[time.Time]string{}
	if err := n.collect(n.dir(), found); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	days := make([]Entry, 0, len(found))
	for d, p := range found {
		days = append(days, Entry{Day: d, Path: p})
	}
	sort.Slice(days, func(i, j int) bool { return days[i].Day.Before(days[j].Day) })
	return days, nil
}

// collect adds the Daily Notes in dir and its subfolders to found.
func (n Notes) collect(dir string, found map[time.Time]string) error {
	entries, err := n.FS.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		if e.IsDir() {
			if err := n.collect(p, found); err != nil {
				return err
			}
			continue
		}
		d, ok := parseName(e.Name())
		if !ok {
			continue
		}
		if _, dup := found[d]; !dup || p == n.Path(d) {
			found[d] = p
		}
	}
	return nil
}

// Prev is the nearest Daily Note before from's day.
func (n Notes) Prev(from time.Time) (Entry, bool, error) {
	days, err := n.days()
	key := from.Format(dates.ISO)
	for i := len(days) - 1; i >= 0; i-- {
		if days[i].Day.Format(dates.ISO) < key {
			return days[i], true, nil
		}
	}
	return Entry{}, false, err
}

// Next is the nearest Daily Note after from's day.
func (n Notes) Next(from time.Time) (Entry, bool, error) {
	days, err := n.days()
	key := from.Format(dates.ISO)
	for _, d := range days {
		if d.Day.Format(dates.ISO) > key {
			return d, true, nil
		}
	}
	return Entry{}, false, err
}

// Ensure makes sure day's Daily Note exists. A new one is filled from the
// Template (empty when there is none), with now for {{time}}, and written
// at once. It returns the Note's path and, when it wrote one, the text.
func (n Notes) Ensure(day, now time.Time) (string, []byte, error) {
	path := n.Path(day)
	if _, err := n.FS.Stat(path); err == nil {
		return path, nil, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return path, nil, err
	}
	tmpl, err := n.FS.ReadFile(filepath.Join(n.Vault, filepath.FromSlash(n.Template)))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return path, nil, fmt.Errorf("reading the daily Template: %w", err)
	}
	data := []byte(Render(string(tmpl), Vars{Day: day, Now: now, Title: day.Format(dates.ISO)}))
	if err := n.FS.MkdirAll(filepath.Dir(path)); err != nil {
		return path, nil, err
	}
	if err := n.FS.WriteFile(path, data); err != nil {
		return path, nil, err
	}
	return path, data, nil
}
