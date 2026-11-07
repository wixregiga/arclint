//go:build !unix

package main

import "errors"

func mkfifo(string) error { return errors.New("named pipes are a unix file type") }
