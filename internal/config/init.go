package config

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/tedkulp/pholio/internal/seam"
)

// dailyTemplate is the starter daily Template that init writes.
const dailyTemplate = "# {{title}}\n\n## Tasks\n"

// vaultKeyDocs describes each Vault key for the commented config init
// writes, in the order they appear there. value renders the key's setting in
// c as TOML.
var vaultKeyDocs = []struct {
	key, help string
	value     func(c Config) string
}{
	{"day_starts_at", "When Today begins; \"04:00\" keeps 1am on the previous day.", func(c Config) string {
		return strconv.Quote(fmt.Sprintf("%02d:%02d", int(c.DayStartsAt.Hours()), int(c.DayStartsAt.Minutes())%60))
	}},
	{"daily_folder", "Folder that holds Daily Notes.", func(c Config) string { return strconv.Quote(c.DailyFolder) }},
	{"daily_template", "Template for new Daily Notes.", func(c Config) string { return strconv.Quote(c.DailyTemplate) }},
	{"daily_subfolder", "Date-based folders for new Daily Notes, such as \"YYYY/MM\"; \"\" is none.", func(c Config) string { return strconv.Quote(c.DailySubfolder) }},
	{"zettel_folder", "Folder that holds Zettels.", func(c Config) string { return strconv.Quote(c.ZettelFolder) }},
	{"new_note_folder", "Folder for new Notes; \"\" is the Vault root.", func(c Config) string { return strconv.Quote(c.NewNoteFolder) }},
	{"tasks_heading", "Heading the Task List adds new Tasks under.", func(c Config) string { return strconv.Quote(c.TasksHeading) }},
	{"task_format", "Task Format for new Task Metadata: \"dataview\" ([due:: 2026-10-10]) or \"emoji\" (📅 2026-10-10).", func(c Config) string { return strconv.Quote(string(c.TaskFormat)) }},
}

// InitVaultConfig is the Vault config init writes: every Vault key
// commented out at its default, so it loads the same as no file.
func InitVaultConfig() string {
	var b strings.Builder
	b.WriteString("# pholio Vault config. Uncomment a key to change it; the values shown are the defaults.\n")
	d := Default()
	for _, k := range vaultKeyDocs {
		fmt.Fprintf(&b, "\n# %s\n# %s = %s\n", k.help, k.key, k.value(d))
	}
	return b.String()
}

// Init makes dir (absolute) a Vault: it creates the folder, the Daily Note
// and Zettel folders, the daily Template and the Vault config, whichever are
// missing, and sets vault in the user config if it is unset. It never
// overwrites a file. It returns one line per thing done, or an error, having
// changed nothing, if dir is a file or inside another Vault.
func Init(fsys seam.FS, d Dirs, home, dir string) ([]string, error) {
	dir = filepath.Clean(dir)
	tilde := func(p string) string { return tildePath(p, home) }
	if info, err := fsys.Stat(dir); err == nil && !info.IsDir() {
		return nil, fmt.Errorf("%s is a file, not a folder", tilde(dir))
	}
	user, _ := LoadUser(fsys, d) // problems show when the Vault is opened
	configured := expandHome(user.Vault, home)
	if outer := enclosingVault(fsys, dir, configured); outer != "" {
		rel, _ := filepath.Rel(outer, dir) // cannot fail: dir is inside outer
		return nil, fmt.Errorf("%s is already a Vault; %s is inside it", tilde(outer), rel)
	}

	var lines []string
	if exists(fsys, VaultConfigPath(dir)) {
		lines = append(lines, tilde(dir)+" is already a Vault")
	}
	cfg := Default()
	for _, folder := range []string{dir, filepath.Join(dir, cfg.DailyFolder), filepath.Join(dir, cfg.ZettelFolder)} {
		if exists(fsys, folder) {
			continue
		}
		if err := fsys.MkdirAll(folder); err != nil {
			return lines, err
		}
		lines = append(lines, "created "+tilde(folder))
	}
	for _, f := range []struct{ path, data string }{
		{filepath.Join(dir, filepath.FromSlash(cfg.DailyTemplate)), dailyTemplate},
		{VaultConfigPath(dir), InitVaultConfig()},
	} {
		created, err := writeNew(fsys, f.path, f.data)
		if err != nil {
			return lines, err
		}
		if created {
			lines = append(lines, "created "+tilde(f.path))
		}
	}

	userPath := UserConfigPath(d)
	want := strconv.Quote(tilde(dir))
	set, parses := userVaultKey(fsys, userPath)
	switch {
	case configured == dir:
	case !parses:
		lines = append(lines, fmt.Sprintf("%s does not parse; set vault = %s in it to use this one", tilde(userPath), want))
	case set:
		lines = append(lines, fmt.Sprintf("vault is already set to %s; set vault = %s in %s to use this one", show(user.Vault), want, tilde(userPath)))
	default:
		if err := appendLine(fsys, userPath, "vault = "+want); err != nil {
			return lines, err
		}
		lines = append(lines, fmt.Sprintf("added vault = %s to %s", want, tilde(userPath)))
	}
	return lines, nil
}

// userVaultKey reports whether the user config sets vault (to anything,
// even ""), and whether it parses. A missing file parses and sets nothing.
// Appending to a file that fails either check would leave TOML that does
// not load.
func userVaultKey(fsys seam.FS, p string) (set, parses bool) {
	data, err := fsys.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return false, true
	}
	var raw map[string]any
	if err != nil || toml.Unmarshal(data, &raw) != nil {
		return false, false
	}
	_, set = raw["vault"]
	return set, true
}

// enclosingVault is the Vault strictly above dir, if any: the configured
// Vault, or the nearest folder holding a Vault marker.
func enclosingVault(fsys seam.FS, dir, configured string) string {
	if configured != "" && configured != dir && within(configured, dir) {
		return configured
	}
	if dir == filepath.Dir(dir) {
		return ""
	}
	return markedVault(fsys, filepath.Dir(dir))
}

func exists(fsys seam.FS, p string) bool {
	_, err := fsys.Stat(p)
	return err == nil
}

// writeNew writes data to p, creating its folder, unless p already exists.
func writeNew(fsys seam.FS, p, data string) (bool, error) {
	if exists(fsys, p) {
		return false, nil
	}
	if err := fsys.MkdirAll(filepath.Dir(p)); err != nil {
		return false, err
	}
	return true, fsys.WriteFile(p, []byte(data))
}

// appendLine adds line to the end of file p, keeping what is there and
// creating the file and its folder if needed.
func appendLine(fsys seam.FS, p, line string) error {
	old, err := fsys.ReadFile(p)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err != nil {
		if err := fsys.MkdirAll(filepath.Dir(p)); err != nil {
			return err
		}
	}
	if len(old) > 0 && old[len(old)-1] != '\n' {
		old = append(old, '\n')
	}
	return fsys.WriteFile(p, append(old, line+"\n"...))
}

// tildePath writes p as ~/… when it is under home.
func tildePath(p, home string) string {
	if p == home {
		return "~"
	}
	if within(home, p) {
		rel, _ := filepath.Rel(home, p)
		return "~/" + filepath.ToSlash(rel)
	}
	return p
}
