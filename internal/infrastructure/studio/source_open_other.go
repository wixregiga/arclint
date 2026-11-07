//go:build !unix

package studio

import "os"

// openSourceFlags opens read-only. The walk refuses a final link before
// the open and checks after it that the file opened is the file it saw,
// and a non-regular file is refused once open.
const openSourceFlags = os.O_RDONLY
