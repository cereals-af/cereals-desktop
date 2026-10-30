//go:build windows

package session

import (
	"os"

	"golang.org/x/sys/windows"
)

// Delete-on-close also removes private log archives after an abnormal exit.
func newArchiveFile() (*os.File, error) {
	temporary, err := os.CreateTemp("", "cereals-session-*.log")
	if err != nil {
		return nil, err
	}
	name := temporary.Name()
	if err = temporary.Close(); err != nil {
		os.Remove(name)
		return nil, err
	}
	filename, err := windows.UTF16PtrFromString(name)
	if err != nil {
		os.Remove(name)
		return nil, err
	}
	handle, err := windows.CreateFile(filename, windows.GENERIC_READ|windows.GENERIC_WRITE|windows.DELETE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_TEMPORARY|windows.FILE_FLAG_DELETE_ON_CLOSE, 0)
	if err != nil {
		os.Remove(name)
		return nil, err
	}
	return os.NewFile(uintptr(handle), name), nil
}
