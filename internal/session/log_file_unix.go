//go:build !windows

package session

import "os"

func newArchiveFile() (*os.File, error) {
	file, err := os.CreateTemp("", "cereals-session-*.log")
	if err != nil {
		return nil, err
	}
	if err = os.Remove(file.Name()); err != nil {
		file.Close()
		os.Remove(file.Name())
		return nil, err
	}
	return file, nil
}
