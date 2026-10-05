//go:build unix

package seam

import (
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

// topTrash finds the trash of the filesystem path is on: $topdir/.Trash-$uid,
// where topdir is that filesystem's mount point. top is topdir.
func topTrash(path string) (dir, top string, ok bool) {
	dev := func(p string) (uint64, bool) {
		var st syscall.Stat_t
		if err := syscall.Lstat(p, &st); err != nil {
			return 0, false
		}
		return uint64(st.Dev), true //nolint:unconvert // Dev is int32 on some platforms
	}
	d, ok := dev(path)
	if !ok {
		return "", "", false
	}
	top = path
	for {
		up := filepath.Dir(top)
		if up == top {
			break
		}
		if ud, ok := dev(up); !ok || ud != d {
			break
		}
		top = up
	}
	return filepath.Join(top, ".Trash-"+strconv.Itoa(os.Getuid())), top, true
}
