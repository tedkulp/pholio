package config

import "github.com/tedkulp/pholio/internal/seam"

// Session is the result of startup: the Vault and file to open, the merged
// config, and the one-line problem message ("" if none).
type Session struct {
	Dirs    Dirs
	Target  Target
	Config  Config
	Message string
}

// Startup finds the Vault from arg (an absolute path, or "") and the user
// config, then loads both config files. The returned error is the one-line
// hint to print when there is no usable Vault.
func Startup(fsys seam.FS, d Dirs, home, arg string) (Session, error) {
	user, _ := LoadUser(fsys, d) // problems are reported by the full load below
	t, err := FindVault(fsys, d, home, arg, user.Vault)
	if err != nil {
		return Session{}, err
	}
	s := Session{Dirs: d, Target: t}
	s.Config, s.Message = s.Reload(fsys)
	return s, nil
}

// Reload rereads the user and Vault config files. F8 calls it. The Vault
// itself does not change for the rest of the run.
func (s Session) Reload(fsys seam.FS) (Config, string) {
	cfg, problems := Load(fsys, s.Dirs, s.Target.Vault)
	return cfg, Message(problems)
}
