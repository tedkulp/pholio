//go:build unix

package watch

import "syscall"

// RaiseFileLimit raises the soft RLIMIT_NOFILE to the hard limit (capped at
// kern.maxfilesperproc on macOS), since kqueue needs one descriptor per
// watched file. Go's os package already tries this at startup; calling it
// again makes the intent explicit and is harmless.
func RaiseFileLimit() error {
	var lim syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &lim); err != nil {
		return err
	}
	if lim.Cur >= lim.Max {
		return nil
	}
	want := lim
	want.Cur = lim.Max
	err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &want)
	if err != nil {
		if c, ok := ceiling(); ok && c > lim.Cur && c < lim.Max {
			want.Cur = c
			err = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &want)
		}
	}
	return err
}
