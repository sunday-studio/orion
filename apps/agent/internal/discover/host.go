package discover

import (
	"os"
	"path/filepath"
)

func dirExists(dir string) bool {
	st, err := os.Stat(dir)
	return err == nil && st.IsDir()
}

func dirReadable(dir string) bool {
	f, err := os.Open(dir)
	if err != nil {
		return false
	}
	f.Close()
	return true
}

func globFiles(pattern string) ([]string, error) { return filepath.Glob(pattern) }
