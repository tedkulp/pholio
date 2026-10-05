package theme

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestSlotConstantsMatchDefaultTheme keeps slots.go and themes/default.toml
// in step: every Slot constant is defined by the default theme, and every
// default slot has a constant.
func TestSlotConstantsMatchDefaultTheme(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "slots.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var consts []string
	ast.Inspect(f, func(n ast.Node) bool {
		vs, ok := n.(*ast.ValueSpec)
		if !ok || vs.Type == nil || vs.Type.(*ast.Ident).Name != "Slot" {
			return true
		}
		for _, v := range vs.Values {
			s, err := strconv.Unquote(v.(*ast.BasicLit).Value)
			if err != nil {
				t.Fatal(err)
			}
			consts = append(consts, s)
		}
		return true
	})
	sort.Strings(consts)

	var slots []string
	for key := range Default().styles {
		slots = append(slots, string(key))
	}
	sort.Strings(slots)

	if strings.Join(consts, "\n") != strings.Join(slots, "\n") {
		t.Errorf("Slot constants and default.toml slots differ:\nconstants: %q\ndefault:   %q", consts, slots)
	}
}
