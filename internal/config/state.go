package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/BurntSushi/toml"

	"github.com/tedkulp/pholio/internal/seam"
)

// State is what pholio remembers between runs. Only the app writes it.
type State struct {
	SidebarWidth int `toml:"sidebar_width"`
}

// DefaultState is the state used before anything has been saved.
func DefaultState() State {
	return State{SidebarWidth: 30}
}

// StateStore reads and writes $XDG_STATE_HOME/pholio/state.toml.
type StateStore struct {
	fsys seam.FS
	path string
}

// NewStateStore returns the store for the state file under d.StateHome.
func NewStateStore(fsys seam.FS, d Dirs) StateStore {
	return StateStore{fsys: fsys, path: filepath.Join(d.StateHome, "pholio", "state.toml")}
}

// Path is the state file's path.
func (s StateStore) Path() string { return s.path }

// Load reads the state. A missing file gives DefaultState and no error. An
// unreadable or invalid file gives DefaultState and an error describing it.
func (s StateStore) Load() (State, error) {
	data, err := s.fsys.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return DefaultState(), nil
	}
	if err != nil {
		return DefaultState(), err
	}
	st := DefaultState()
	if _, err := toml.Decode(string(data), &st); err != nil {
		return DefaultState(), fmt.Errorf("%s: %w", s.path, err)
	}
	if st.SidebarWidth < 1 {
		return DefaultState(), fmt.Errorf("%s: sidebar_width: want a positive number, got %d", s.path, st.SidebarWidth)
	}
	return st, nil
}

// Save writes st, creating the state folder if needed.
func (s StateStore) Save(st State) error {
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(st); err != nil {
		return err
	}
	if err := s.fsys.MkdirAll(filepath.Dir(s.path)); err != nil {
		return err
	}
	return s.fsys.WriteFile(s.path, buf.Bytes())
}
