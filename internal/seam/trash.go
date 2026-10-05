package seam

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

// SystemTrash is the real Trash. It follows the FreeDesktop.org Trash
// specification: an item moves to Dir/files and a Dir/info/<name>.trashinfo
// file records where it came from and when, so file managers can restore
// it. An item on another filesystem goes to that filesystem's
// $topdir/.Trash-$uid instead, where the platform can tell (see topTrash).
type SystemTrash struct {
	// Dir is the home trash, $XDG_DATA_HOME/Trash.
	Dir string
	// Clock stamps DeletionDate; nil means the system clock.
	Clock Clock
	// NoInfo skips the info files and the files/ subfolder: macOS's
	// ~/.Trash holds the items themselves.
	NoInfo bool
}

var _ Trash = SystemTrash{}

// NewSystemTrash returns the user's trash: $XDG_DATA_HOME/Trash (by default
// ~/.local/share/Trash), or ~/.Trash on macOS.
func NewSystemTrash(getenv func(string) string, home string, clock Clock) SystemTrash {
	if runtime.GOOS == "darwin" {
		return SystemTrash{Dir: filepath.Join(home, ".Trash"), Clock: clock, NoInfo: true}
	}
	data := getenv("XDG_DATA_HOME")
	if data == "" || !filepath.IsAbs(data) {
		data = filepath.Join(home, ".local", "share")
	}
	return SystemTrash{Dir: filepath.Join(data, "Trash"), Clock: clock}
}

// Trash moves the file or folder at path to the trash.
func (t SystemTrash) Trash(path string) error {
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(path); err != nil {
		return err
	}
	err = t.moveTo(t.Dir, path, path)
	if !errors.Is(err, syscall.EXDEV) {
		return err
	}
	// Another filesystem: its own trash, with Path relative to its top.
	dir, top, ok := topTrash(path)
	if !ok {
		return fmt.Errorf("%s is on another filesystem than the trash: %w", path, err)
	}
	rel, rerr := filepath.Rel(top, path)
	if rerr != nil {
		return rerr
	}
	return t.moveTo(dir, path, rel)
}

// moveTo moves path into the trash folder dir, recording recorded as its
// original location.
func (t SystemTrash) moveTo(dir, path, recorded string) error {
	files, info := filepath.Join(dir, "files"), filepath.Join(dir, "info")
	if t.NoInfo {
		files = dir
	}
	if err := os.MkdirAll(files, 0o700); err != nil {
		return err
	}
	if !t.NoInfo {
		if err := os.MkdirAll(info, 0o700); err != nil {
			return err
		}
	}
	name, infoFile, err := t.reserve(files, info, filepath.Base(path), recorded)
	if err != nil {
		return err
	}
	if err := os.Rename(path, filepath.Join(files, name)); err != nil {
		if infoFile != "" {
			_ = os.Remove(infoFile)
		}
		return err
	}
	return nil
}

// reserve picks a name not yet used in the trash and, unless NoInfo,
// claims it by creating its info file (O_EXCL makes that atomic).
func (t SystemTrash) reserve(files, info, base, recorded string) (name, infoFile string, err error) {
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	if stem == "" { // a dotfile such as ".hidden"
		stem, ext = base, ""
	}
	for n := 1; n < 10000; n++ {
		name = base
		if n > 1 {
			name = stem + "." + strconv.Itoa(n) + ext
		}
		if _, err := os.Lstat(filepath.Join(files, name)); !errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if t.NoInfo {
			return name, "", nil
		}
		infoFile = filepath.Join(info, name+".trashinfo")
		f, err := os.OpenFile(infoFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", "", err
		}
		_, werr := f.WriteString(t.infoText(recorded))
		if cerr := f.Close(); werr == nil {
			werr = cerr
		}
		if werr != nil {
			_ = os.Remove(infoFile)
			return "", "", werr
		}
		return name, infoFile, nil
	}
	return "", "", fmt.Errorf("no free name in the trash for %s", base)
}

// infoText is the .trashinfo contents for an item from path.
func (t SystemTrash) infoText(path string) string {
	clock := t.Clock
	if clock == nil {
		clock = SystemClock{}
	}
	u := url.URL{Path: filepath.ToSlash(path)}
	return "[Trash Info]\nPath=" + u.EscapedPath() + "\nDeletionDate=" +
		clock.Now().Format("2006-01-02T15:04:05") + "\n"
}
