//go:build !unix

package watch

// RaiseFileLimit does nothing where there is no RLIMIT_NOFILE.
func RaiseFileLimit() error { return nil }
