//go:build windows

package hooks

import (
	"os"
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
