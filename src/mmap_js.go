//go:build js

package src

import (
	"io"
	"os"
)

// mapFile emulates memory mapping in the browser, where mmap is not available:
// the whole file is read into memory and written back on release.
func mapFile(file *os.File, writable bool) ([]byte, func() error, error) {
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, nil, err
	}

	release := func() error {
		if !writable {
			return nil
		}
		_, err := file.WriteAt(data, 0)
		return err
	}

	return data, release, nil
}
