package engine

import "strconv"

// setDot remembers p as the change "." repeats. When p entered insert mode,
// the keys typed until esc are appended as they arrive.
func (e *Engine) setDot(p parsed) {
	if e.replaying {
		return
	}
	e.dot = append([]string(nil), p.rest...)
	e.dotCount, e.dotReg = p.count, p.reg
	e.recording = e.Mode == Insert
}

// repeatDot replays the last change. A count replaces the original one.
func (e *Engine) repeatDot(count int) {
	if e.dot == nil {
		return
	}
	if count == 0 {
		count = e.dotCount
	}
	var keys []string
	if e.dotReg != 0 {
		keys = append(keys, `"`, string(e.dotReg))
	}
	if count > 0 {
		for _, r := range strconv.Itoa(count) {
			keys = append(keys, string(r))
		}
	}
	keys = append(keys, e.dot...)
	e.replaying = true
	defer func() { e.replaying = false }()
	for _, k := range keys {
		e.Feed(k)
	}
	if e.Mode == Insert {
		e.Feed("esc")
	}
}
