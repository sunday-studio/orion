package appres

import (
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// dirSize sums regular file sizes under root, stopping at budget. The result
// is then a lower bound and capped is true. Symlinks are not followed and
// unreadable entries are skipped.
func dirSize(root string, budget time.Duration) (size uint64, capped bool, err error) {
	if _, err := os.Stat(root); err != nil {
		return 0, false, err
	}
	deadline := time.Now().Add(budget)
	stopped := errStop{}
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // permission denied etc.: skip
		}
		if time.Now().After(deadline) {
			capped = true
			return stopped
		}
		if d.Type().IsRegular() {
			if info, err := d.Info(); err == nil {
				size += uint64(info.Size())
			}
		}
		return nil
	})
	if walkErr != nil && walkErr != error(stopped) {
		return size, capped, walkErr
	}
	return size, capped, nil
}

type errStop struct{}

func (errStop) Error() string { return "walk budget exceeded" }
