//go:build !windows

package winsym

// retry runs op once: only Windows has a scanner that briefly holds a just-written file.
func retry(op func() error) error { return op() }
