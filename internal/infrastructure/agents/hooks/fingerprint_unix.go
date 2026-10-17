//go:build unix

package hooks

import (
	"time"

	"golang.org/x/sys/unix"
)

// fingerprintOf reads the file at path without following a link. Its
// change time moves on every write and on every change of mode, so it
// stands for the mode as well.
func fingerprintOf(path string) fingerprint {
	var status unix.Stat_t
	if err := unix.Lstat(path, &status); err != nil {
		return fingerprint{Absent: true}
	}
	return fingerprint{Size: status.Size, Modified: status.Mtim.Nano(), Changed: status.Ctim.Nano()}
}

// directoryStamp reads when the directory at path last changed, following
// a link, as a removal from it stamps it: the project root may be reached
// through a link.
func directoryStamp(path string) (time.Time, bool) {
	var status unix.Stat_t
	if err := unix.Stat(path, &status); err != nil {
		return time.Time{}, false
	}
	return time.Unix(0, status.Ctim.Nano()), true
}
