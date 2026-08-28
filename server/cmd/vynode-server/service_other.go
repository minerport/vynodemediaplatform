//go:build !windows

package main

import (
	"io"
	"os"
)

func runWindowsService() (bool, error)                   { return false, nil }
func windowsLogWriter(string) (io.Writer, func(), error) { return os.Stdout, func() {}, nil }
