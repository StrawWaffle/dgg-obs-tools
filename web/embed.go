// Package web carries the built pages inside the executable, so the app is a
// single file. Run `npm run build` in this directory before `go build`.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Pages is the built web app.
func Pages() fs.FS {
	pages, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	return pages
}
