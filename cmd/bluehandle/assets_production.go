//go:build windows && production

package main

import (
	"embed"
	"io/fs"
)

//go:embed all:frontend/dist
var dist embed.FS

// frontendFS serves the built frontend embedded in the executable.
func frontendFS() fs.FS {
	sub, err := fs.Sub(dist, "frontend/dist")
	if err != nil {
		panic(err)
	}
	return sub
}
