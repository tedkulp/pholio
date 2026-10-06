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
	// Device IDs are only compared, so they stay in Stat_t's own field type
	// (uint64 on Linux, int32 on macOS).
	stat := func(p string) (syscall.Stat_t, bool) {
		var st syscall.Stat_t
		return st, syscall.Lstat(p, &st) == nil
	}
	st, ok := stat(path)
	if !ok {
		return "", "", false
	}
	top = path
	for {
		up := filepath.Dir(top)
		if up == top {
			break
		}
		if ust, ok := stat(up); !ok || ust.Dev != st.Dev {
			break
		}
		top = up
	}
	return filepath.Join(top, ".Trash-"+strconv.Itoa(os.Getuid())), top, true
}
