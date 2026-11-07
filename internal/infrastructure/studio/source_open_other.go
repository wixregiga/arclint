//go:build !unix

package studio

import "os"

// openSourceFlags opens read-only; the links checks before and after
// the open refuse a link, and a non-regular file is refused once open.
const openSourceFlags = os.O_RDONLY
