// Command arclint enforces architecture contracts over a repository.
// Exit codes: 0 clean, 1 error-severity findings, 2 configuration or
// usage error.
package main

import "os"

var version = "0.1.0"

func main() {
	os.Exit(run(os.Args[1:]))
}
