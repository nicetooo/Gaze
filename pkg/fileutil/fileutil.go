// Package fileutil provides shared filesystem helpers for safe persistence.
package fileutil

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// WriteFileAtomic writes data to path atomically: it writes to a temporary
// file in the same directory, fsyncs it, then renames it over the target.
// A crash or full disk can never leave a truncated file at path.
//
// The temp file name appends ".tmp-<random>" to the target's base name, so it
// never ends in ".json" — directory watchers that filter on that suffix (e.g.
// WorkflowWatcher) only ever observe the final rename of the real file.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() {
		tmp.Close()
		os.Remove(tmpName)
	}
	if _, err := tmp.Write(data); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return err
	}
	// CreateTemp uses 0600; align with the requested permissions
	if err := tmp.Chmod(perm); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}

// QuarantineCorrupt renames a file that failed to parse to
// "<path>.corrupt-<unix-ts>" so the on-disk evidence is preserved and a
// subsequent full-overwrite save cannot silently destroy it.
// Returns the quarantine path.
func QuarantineCorrupt(path string) (string, error) {
	target := fmt.Sprintf("%s.corrupt-%d", path, time.Now().Unix())
	if err := os.Rename(path, target); err != nil {
		return "", err
	}
	return target, nil
}
