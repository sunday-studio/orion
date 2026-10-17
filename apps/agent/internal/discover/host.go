package discover

import "path/filepath"

func globFiles(pattern string) ([]string, error) { return filepath.Glob(pattern) }
