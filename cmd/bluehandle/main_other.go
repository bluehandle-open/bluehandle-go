//go:build !windows

// Command bluehandle is the desktop app. Only Windows is supported so far;
// use cmd/bluehandle-cli elsewhere.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "bluehandle: the desktop app only supports Windows for now; use bluehandle-cli")
	os.Exit(1)
}
