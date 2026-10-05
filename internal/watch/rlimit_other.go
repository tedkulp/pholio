//go:build unix && !darwin

package watch

// ceiling has no separate per-process cap outside macOS.
func ceiling() (uint64, bool) { return 0, false }
