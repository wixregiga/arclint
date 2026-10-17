//go:build windows

package hooks

import (
	"os"
	"time"
)

// fingerprintOf reads the file at path without following a link. Windows
// file information records no change time, so a copy that keeps the
// source's size, modification time and mode is not told apart there.
func fingerprintOf(path string) fingerprint {
	info, err := os.Lstat(path)
	if err != nil {
		return fingerprint{Absent: true}
	}
	return fingerprint{Size: info.Size(), Modified: info.ModTime().UnixNano(), Mode: uint32(info.Mode())}
}

// directoryStamp reads when the directory at path last changed, following
// a link, as a removal from it stamps it: the project root may be reached
// through a link.
func directoryStamp(path string) (time.Time, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, false
	}
	return info.ModTime(), true
}
