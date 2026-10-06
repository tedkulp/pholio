//go:build unix

package watch_test

import (
	"syscall"
	"testing"

	"github.com/tedkulp/pholio/internal/watch"
)

func TestRaiseFileLimit(t *testing.T) {
	var lim syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &lim); err != nil {
		t.Fatal(err)
	}
	orig := lim
	lim.Cur = min(64, lim.Max)
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &lim); err != nil {
		t.Skip("can't lower the limit:", err)
	}
	t.Cleanup(func() { _ = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &orig) })

	_ = watch.RaiseFileLimit()
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &lim); err != nil {
		t.Fatal(err)
	}
	if lim.Cur <= 64 && lim.Max > 64 {
		t.Errorf("soft limit still %d (hard %d)", lim.Cur, lim.Max)
	}
}
