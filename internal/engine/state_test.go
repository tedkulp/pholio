package engine

import (
	"os/exec"
	"strings"
	"testing"
)

func TestPendingKeys(t *testing.T) {
	cases := []struct {
		keys, pending string
		opPending     bool
	}{
		{"", "", false},
		{"2", "2", false},
		{`"a`, `"a`, false},
		{`"a2d`, `"a2d`, true},
		{"d3", "d3", true},
		{"di", "di", true},
		{"f", "f", false},
		{"df", "df", true},
		{"g", "g", false},
		{"dw", "", false},
		{"dz", "", false},
		{`"<esc>`, "", false},
		{"r<esc>", "", false},
		{"ix", "", false},
	}
	for _, c := range cases {
		e := load("|abc def")
		feed(e, c.keys)
		if e.PendingKeys() != c.pending || e.OperatorPending() != c.opPending {
			t.Errorf("%q: pending %q op %v, want %q %v",
				c.keys, e.PendingKeys(), e.OperatorPending(), c.pending, c.opPending)
		}
	}
}

func TestMessageClearsOnNextCommand(t *testing.T) {
	e := load("|a")
	feed(e, "u")
	if e.Msg == "" {
		t.Fatal("expected a message after u")
	}
	feed(e, "l")
	if e.Msg != "" {
		t.Errorf("msg = %q after next command", e.Msg)
	}
}

func TestLinesYankedMessage(t *testing.T) {
	e := load("|a\nb\nc")
	feed(e, "3yy")
	if e.Msg != "3 lines yanked" {
		t.Errorf("msg = %q", e.Msg)
	}
}

func TestCellWidth(t *testing.T) {
	cases := []struct {
		g    string
		col  int
		want int
	}{
		{"a", 0, 1},
		{"日", 0, 2},
		{"👍🏽", 0, 2},
		{"é́", 0, 1},
		{"\t", 0, 4},
		{"\t", 1, 3},
		{"\t", 4, 4},
	}
	for _, c := range cases {
		if got := CellWidth(c.g, c.col); got != c.want {
			t.Errorf("CellWidth(%q, %d) = %d, want %d", c.g, c.col, got, c.want)
		}
	}
	if got := Cells("a\t日x", len("a\t日")); got != 6 {
		t.Errorf("Cells = %d, want 6", got)
	}
}

func TestNewBuffer(t *testing.T) {
	cases := []struct {
		in    string
		lines int
		out   string
	}{
		{"", 1, "\n"},
		{"a", 1, "a\n"},
		{"a\n", 1, "a\n"},
		{"a\r\nb\r\n", 2, "a\nb\n"},
		{"a\n\n", 2, "a\n\n"},
	}
	for _, c := range cases {
		b := NewBuffer(c.in)
		if b.LineCount() != c.lines || b.String() != c.out {
			t.Errorf("NewBuffer(%q): %d lines %q, want %d %q", c.in, b.LineCount(), b.String(), c.lines, c.out)
		}
	}
}

func TestNoBubbleTeaImport(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", ".").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	if !strings.Contains(string(out), "uax29") {
		t.Fatalf("go list output does not look like engine's deps:\n%s", out)
	}
	for dep := range strings.FieldsSeq(string(out)) {
		if strings.Contains(dep, "bubbletea") || strings.Contains(dep, "lipgloss") {
			t.Errorf("engine depends on %s; it must not import Bubble Tea", dep)
		}
	}
}
