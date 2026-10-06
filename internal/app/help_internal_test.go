package app

import (
	"slices"
	"testing"
)

func TestEveryBindingHasHelp(t *testing.T) {
	for _, b := range bindings {
		if b.help == "" {
			t.Errorf("binding %q (scope %d, %s) has no help text", b.key, b.scope, b.action)
		}
	}
}

func TestEveryScopeIsInHelp(t *testing.T) {
	for _, b := range bindings {
		if !slices.ContainsFunc(helpScopes, func(h helpScope) bool { return h.scope == b.scope }) {
			t.Errorf("binding %q: its scope %d is not in helpScopes", b.key, b.scope)
		}
	}
}
