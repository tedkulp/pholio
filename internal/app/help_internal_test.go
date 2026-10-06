package app

import "testing"

func TestEveryBindingHasHelp(t *testing.T) {
	for _, b := range bindings {
		if b.help == "" {
			t.Errorf("binding %q (scope %d, %s) has no help text", b.key, b.scope, b.action)
		}
	}
}
