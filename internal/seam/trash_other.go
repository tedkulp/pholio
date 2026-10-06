//go:build !unix

package seam

// topTrash is unsupported here: an item on another filesystem than the
// home trash is refused.
func topTrash(string) (dir, top string, ok bool) { return "", "", false }
