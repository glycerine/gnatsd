package swp

import (
	"os"
)

// fileExists returns true if the named path
// exists in the filesystem and is a file (and
// not a directory).
func fileExists(name string) bool {
	fi, err := os.Stat(name)
	if err != nil {
		return false
	}
	if fi.IsDir() {
		return false
	}
	return true
}

// DirExists returns true if the named path
// is a directly presently in the filesystem.
func dirExists(name string) bool {
	fi, err := os.Stat(name)
	if err != nil {
		return false
	}
	if fi.IsDir() {
		return true
	}
	return false
}
