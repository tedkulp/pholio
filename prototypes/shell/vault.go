package main

// PROTOTYPE: file tree and Task scanning over a scratch copy of the sample vault.

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type node struct {
	path, name  string
	dir, hidden bool
	depth       int
}

func (m *model) visibleNodes() []node {
	var out []node
	var walk func(dir string, depth int)
	walk = func(dir string, depth int) {
		ents, _ := os.ReadDir(dir)
		sort.SliceStable(ents, func(i, j int) bool {
			if ents[i].IsDir() != ents[j].IsDir() {
				return ents[i].IsDir()
			}
			return strings.ToLower(ents[i].Name()) < strings.ToLower(ents[j].Name())
		})
		for _, e := range ents {
			hidden := strings.HasPrefix(e.Name(), ".")
			if hidden && !m.showHidden {
				continue
			}
			p := filepath.Join(dir, e.Name())
			out = append(out, node{path: p, name: e.Name(), dir: e.IsDir(), hidden: hidden, depth: depth})
			if e.IsDir() && m.expanded[p] {
				walk(p, depth+1)
			}
		}
	}
	walk(m.vault, 0)
	return out
}

type task struct {
	file      string // vault-relative
	line      int
	text      string
	due, pri  string
	group     int
}

var (
	taskRe = regexp.MustCompile(`^\s*[-*] \[( |x|X)\] (.*)$`)
	dueRe  = regexp.MustCompile(`\bdue:(\d{4}-\d{2}-\d{2})\b`)
	priRe  = regexp.MustCompile(`\bpri:(\w+)\b`)
)

const (
	gOverdue = iota
	gToday
	gUpcoming
	gNoDate
)

var groupNames = []string{"Overdue", "Today", "Upcoming", "No date"}

// scanTasks gathers open Tasks, sorted by group then due date.
func scanTasks(vault string) []task {
	today := time.Now().Format("2006-01-02")
	var ts []task
	filepath.WalkDir(vault, func(p string, d os.DirEntry, err error) error {
		if d.IsDir() && strings.HasPrefix(d.Name(), ".") && p != vault {
			return filepath.SkipDir
		}
		if d.IsDir() && d.Name() == "templates" {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(p, ".md") {
			return nil
		}
		f, err := os.Open(p)
		if err != nil {
			return nil
		}
		defer f.Close()
		rel, _ := filepath.Rel(vault, p)
		sc := bufio.NewScanner(f)
		for n := 0; sc.Scan(); n++ {
			mt := taskRe.FindStringSubmatch(sc.Text())
			if mt == nil || mt[1] != " " {
				continue
			}
			t := task{file: rel, line: n, text: mt[2], group: gNoDate}
			if d := dueRe.FindStringSubmatch(t.text); d != nil {
				t.due = d[1]
				switch {
				case t.due < today:
					t.group = gOverdue
				case t.due == today:
					t.group = gToday
				default:
					t.group = gUpcoming
				}
			}
			if p := priRe.FindStringSubmatch(t.text); p != nil {
				t.pri = p[1]
			}
			ts = append(ts, t)
		}
		return nil
	})
	sort.SliceStable(ts, func(i, j int) bool {
		if ts[i].group != ts[j].group {
			return ts[i].group < ts[j].group
		}
		return ts[i].due < ts[j].due
	})
	return ts
}

func copyDir(src, dst string) {
	filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		rel, _ := filepath.Rel(src, p)
		t := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(t, 0o755)
		}
		b, _ := os.ReadFile(p)
		return os.WriteFile(t, b, 0o644)
	})
}
