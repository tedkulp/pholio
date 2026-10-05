package watch

import "syscall"

// ceiling is the most descriptors a macOS process may have open.
func ceiling() (uint64, bool) {
	n, err := syscall.SysctlUint32("kern.maxfilesperproc")
	return uint64(n), err == nil
}
