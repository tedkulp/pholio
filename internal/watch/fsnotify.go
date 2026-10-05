package watch

import (
	"errors"

	"github.com/fsnotify/fsnotify"

	"github.com/tedkulp/pholio/internal/seam"
)

// fsWatcher adapts fsnotify to seam.Watcher. A kernel queue overflow is
// reported as an OpOverflow event; Chmod-only events are dropped.
type fsWatcher struct {
	w      *fsnotify.Watcher
	events chan seam.WatchEvent
	errs   chan error
	done   chan struct{}
}

// NewFSNotify returns the real seam.Watcher (inotify on Linux, kqueue on
// macOS). It is not recursive: add each directory.
func NewFSNotify() (seam.Watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	f := &fsWatcher{w: w, events: make(chan seam.WatchEvent, 64), errs: make(chan error, 8), done: make(chan struct{})}
	go f.pump()
	return f, nil
}

func (f *fsWatcher) pump() {
	defer close(f.events)
	defer close(f.errs)
	events, errs := f.w.Events, f.w.Errors
	for events != nil || errs != nil {
		select {
		case ev, ok := <-events:
			if !ok {
				events = nil
				continue
			}
			if op := convert(ev.Op); op != 0 {
				f.deliver(seam.WatchEvent{Path: ev.Name, Op: op}, nil)
			}
		case err, ok := <-errs:
			if !ok {
				errs = nil
				continue
			}
			if errors.Is(err, fsnotify.ErrEventOverflow) {
				f.deliver(seam.WatchEvent{Op: seam.OpOverflow}, nil)
			} else {
				f.deliver(seam.WatchEvent{}, err)
			}
		}
	}
}

// deliver hands on an event, or err if set, unless the watcher is closed
// and nobody is reading any more.
func (f *fsWatcher) deliver(ev seam.WatchEvent, err error) {
	if err != nil {
		select {
		case f.errs <- err:
		case <-f.done:
		}
		return
	}
	select {
	case f.events <- ev:
	case <-f.done:
	}
}

func convert(op fsnotify.Op) seam.Op {
	var out seam.Op
	if op.Has(fsnotify.Create) {
		out |= seam.OpCreate
	}
	if op.Has(fsnotify.Write) {
		out |= seam.OpWrite
	}
	if op.Has(fsnotify.Remove) {
		out |= seam.OpRemove
	}
	if op.Has(fsnotify.Rename) {
		out |= seam.OpRename
	}
	return out
}

func (f *fsWatcher) Add(dir string) error           { return f.w.Add(dir) }
func (f *fsWatcher) Remove(dir string) error        { return f.w.Remove(dir) }
func (f *fsWatcher) Events() <-chan seam.WatchEvent { return f.events }
func (f *fsWatcher) Errors() <-chan error           { return f.errs }

// Close stops the watcher; its channels close once pending events drain.
func (f *fsWatcher) Close() error {
	select {
	case <-f.done:
		return nil
	default:
		close(f.done)
	}
	return f.w.Close()
}
