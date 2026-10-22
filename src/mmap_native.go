//go:build !js

package src

import (
	"os"

	"github.com/edsrzf/mmap-go"
)

// mapFile memory-maps the whole file. The returned release function
// flushes changes (when writable) and unmaps the file.
func mapFile(file *os.File, writable bool) ([]byte, func() error, error) {
	mode := mmap.RDONLY
	if writable {
		mode = mmap.RDWR
	}

	data, err := mmap.Map(file, mode, 0)
	if err != nil {
		return nil, nil, err
	}

	release := func() error {
		if writable {
			if err := data.Flush(); err != nil {
				return err
			}
		}
		return data.Unmap()
	}

	return data, release, nil
}
