//go:build windows && !production

package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
)

// frontendFS serves frontend/dist from the source tree, so development
// builds need no embed and vet/test work without building the frontend.
// Run "npm run build" in cmd/bluehandle/frontend first.
func frontendFS() fs.FS {
	_, file, _, _ := runtime.Caller(0)
	return os.DirFS(filepath.Join(filepath.Dir(file), "frontend", "dist"))
}
