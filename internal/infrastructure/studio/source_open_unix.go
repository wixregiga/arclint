//go:build unix

package studio

import (
	"os"
	"syscall"
)

// openSourceFlags opens without following a final link and without
// blocking on a pipe or device, whose type is refused once it is open.
const openSourceFlags = os.O_RDONLY | syscall.O_NOFOLLOW | syscall.O_NONBLOCK
