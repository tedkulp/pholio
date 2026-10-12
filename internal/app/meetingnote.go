package app

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/tedkulp/pholio/internal/calendar"
	"github.com/tedkulp/pholio/internal/daily"
	"github.com/tedkulp/pholio/internal/dates"
	"github.com/tedkulp/pholio/internal/engine"
	"github.com/tedkulp/pholio/internal/index"
	"github.com/tedkulp/pholio/internal/tasks"
)

// meetingTemplate is the built-in meeting Template, used when
// meeting_template is missing.
const meetingTemplate = "# {{title}}\n\n{{start}}–{{end}} · {{daily}}\nAttendees: {{attendees}}\nLocation: {{location}}\nLink: {{link}}\n\n## Notes\n"

// meetingUID is a Meeting's identity as written in its Meeting Note's
// meeting_uid: line: its UID and its start in UTC.
func meetingUID(mt calendar.Meeting) string {
	return mt.UID + "/" + mt.Start.UTC().Format("20060102T150405Z")
}

// meetingNotes maps the meeting_uid of each Meeting Note in meetings_folder
// to its path.
// ponytail: reads every Note in the folder (not subfolders); use the index's
// frontmatter if this gets slow.
func (m Model) meetingNotes() map[string]string {
	dir := m.abs(m.config().MeetingsFolder)
	entries, _ := m.deps.FS.ReadDir(dir) // no folder: no Meeting Notes
	notes := map[string]string{}
	for _, e := range entries {
		if e.IsDir() || !index.IsNote(e.Name()) {
			continue
		}
		p := filepath.Join(dir, e.Name())
		if data, err := m.deps.FS.ReadFile(p); err == nil {
			if uid, ok := frontmatterMeetingUID(string(data)); ok {
				notes[uid] = p
			}
		}
	}
	return notes
}

// frontmatterMeetingUID is the meeting_uid: in a Note's frontmatter, if any.
func frontmatterMeetingUID(s string) (string, bool) {
	lines := strings.Split(s, "\n")
	if strings.TrimRight(lines[0], "\r") != "---" {
		return "", false
	}
	for _, l := range lines[1:] {
		l = strings.TrimRight(l, "\r")
		if l == "---" {
			break
		}
		if v, ok := strings.CutPrefix(l, "meeting_uid:"); ok {
			v = strings.TrimSpace(v)
			if u, err := strconv.Unquote(v); err == nil {
				return u, true
			}
			return v, true
		}
	}
	return "", false
}

// openMeeting opens mt's Meeting Note, creating it first if it has none.
func (m Model) openMeeting(mt calendar.Meeting) (Model, tea.Cmd) {
	if p, ok := m.meetingNotes()[meetingUID(mt)]; ok {
		return m.openNote(p)
	}
	return m.createMeetingNote(mt)
}

// meetingRel is the Vault-relative path for mt's new Meeting Note:
// <meetings_folder>/YYYY-MM-DD-HHmm-<slug>.md, counting up -2, -3, … past
// files that already exist.
func (m Model) meetingRel(mt calendar.Meeting) string {
	s := slug(mt.Title)
	if s == "" {
		s = "meeting"
	}
	base := path.Join(m.config().MeetingsFolder, mt.Start.In(m.now().Location()).Format("2006-01-02-1504")+"-"+s)
	rel := base + ".md"
	for n := 2; ; n++ {
		if _, err := m.deps.FS.Stat(m.abs(rel)); err != nil {
			return rel
		}
		rel = fmt.Sprintf("%s-%d.md", base, n)
	}
}

// meetingNoteText is mt's Meeting Note made from tmpl, with its
// meeting_uid: line in the frontmatter.
func (m Model) meetingNoteText(tmpl string, mt calendar.Meeting) string {
	now := m.now()
	start := mt.Start.In(now.Location())
	title := mt.Title
	if title == "" {
		title = "Untitled meeting"
	}
	text := daily.Render(tmpl, daily.Vars{
		Day:   start,
		Now:   now,
		Title: title,
		Times: map[string]time.Time{"start": start, "end": mt.End.In(now.Location())},
		Text: map[string]string{
			"daily":       "[[" + start.Format(dates.ISO) + "]]",
			"attendees":   strings.Join(mt.Attendees, ", "),
			"location":    mt.Location,
			"link":        mt.Link,
			"description": mt.Description,
		},
	})
	uid := "meeting_uid: " + strconv.Quote(meetingUID(mt)) + "\n"
	if rest, ok := strings.CutPrefix(text, "---\n"); ok {
		return "---\n" + uid + rest
	}
	return "---\n" + uid + "---\n" + text
}

// createMeetingNote writes mt's Meeting Note from the meeting Template
// once the open buffer has been dealt with, and opens it on its last line.
func (m Model) createMeetingNote(mt calendar.Meeting) (Model, tea.Cmd) {
	tmpl := meetingTemplate
	data, err := m.deps.FS.ReadFile(m.abs(m.config().MeetingTemplate))
	switch {
	case err == nil:
		tmpl = string(data)
	case !errors.Is(err, fs.ErrNotExist):
		return m.say("reading the meeting Template: "+err.Error(), true), nil
	}
	rel := m.meetingRel(mt)
	p := m.abs(rel)
	text := []byte(m.meetingNoteText(tmpl, mt))
	start := mt.Start.In(m.now().Location())
	noDaily := ""
	return m.openThen(p, func(m Model) error {
		if err := m.deps.FS.MkdirAll(filepath.Dir(p)); err != nil {
			return fmt.Errorf("creating Meeting Note: %w", err)
		}
		if err := m.deps.FS.WriteFile(p, text); err != nil {
			return fmt.Errorf("creating Meeting Note: %w", err)
		}
		if m.deps.Watch != nil {
			m.deps.Watch.Wrote(p, text)
		}
		if ix := m.index(); ix != nil {
			ix.Update(p, text)
		}
		linked, err := m.linkMeetingNote(start, "- "+start.Format("15:04")+" [["+linkName(rel)+"]]")
		if !linked {
			noDaily = " (no Daily Note for " + start.Format(dates.ISO) + ")"
		}
		return err
	}, func(m Model) Model {
		e := m.ed.Engine()
		e.SetCursor(engine.Pos{Line: e.Buf.LineCount() - 1})
		return m.say("Created "+rel+noDaily, false)
	})
}

// linkMeetingNote appends line, a Meeting Note's back-link, under
// meetings_heading in day's Daily Note, adding the heading when it is
// missing. linked is false when that Daily Note does not exist.
//
// It runs once the open buffer has been dealt with (saved or discarded)
// and just before switching to the Meeting Note, so it always writes the
// file, even when the Daily Note is the open Note.
func (m Model) linkMeetingNote(day time.Time, line string) (linked bool, err error) {
	path := m.dailyNotes().Path(day)
	if _, err := m.deps.FS.Stat(path); errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	heading := m.config().MeetingsHeading
	err = tasks.EditFile(m.otherFile(), path, func(lines []string) []string {
		at := tasks.SectionAt(lines, heading, func(l string) bool { return strings.TrimSpace(l) != "" })
		if at < 0 {
			if n := len(lines); n > 0 && strings.TrimSpace(lines[n-1]) != "" {
				lines = append(lines, "")
			}
			return append(lines, heading, line)
		}
		return slices.Insert(lines, at, line)
	})
	if err != nil {
		return true, fmt.Errorf("linking the Meeting Note from %s: %w", m.rel(path), err)
	}
	return true, nil
}
