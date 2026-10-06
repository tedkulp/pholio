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
}

func (n Notes) dir() string { return filepath.Join(n.Vault, filepath.FromSlash(n.Folder)) }

// Path is where day's Daily Note lives.
func (n Notes) Path(day time.Time) string {
	return filepath.Join(n.dir(), day.Format(dates.ISO)+".md")
}

// Day reports the day of the Daily Note at path, or false when path is not
// a Daily Note: a YYYY-MM-DD.md file directly in the daily folder.
func (n Notes) Day(path string) (time.Time, bool) {
	if filepath.Dir(filepath.Clean(path)) != filepath.Clean(n.dir()) {
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

// days lists the days that have a Daily Note, oldest first. A missing
// daily folder has none.
func (n Notes) days() ([]time.Time, error) {
	entries, err := n.FS.ReadDir(n.dir())
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var days []time.Time
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if d, ok := parseName(e.Name()); ok {
			days = append(days, d)
		}
	}
	sort.Slice(days, func(i, j int) bool { return days[i].Before(days[j]) })
	return days, nil
}

// Prev is the nearest day before from that has a Daily Note.
func (n Notes) Prev(from time.Time) (time.Time, bool, error) {
	days, err := n.days()
	key := from.Format(dates.ISO)
	for i := len(days) - 1; i >= 0; i-- {
		if days[i].Format(dates.ISO) < key {
			return days[i], true, nil
		}
	}
	return time.Time{}, false, err
}

// Next is the nearest day after from that has a Daily Note.
func (n Notes) Next(from time.Time) (time.Time, bool, error) {
	days, err := n.days()
	key := from.Format(dates.ISO)
	for _, d := range days {
		if d.Format(dates.ISO) > key {
			return d, true, nil
		}
	}
	return time.Time{}, false, err
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
