// Package web embeds the built React SPA (web/dist) so the Go binary serves it
// with no external assets. Run `npm --prefix web run build` (or the Docker /
// CI build stage) to populate dist before `go build`; a committed placeholder
// keeps the embed valid when the frontend has not been built locally.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

// Dist returns the built SPA's file system rooted at dist/, or false when the
// frontend has not been built (only the placeholder is present).
func Dist() (fs.FS, bool) {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return nil, false
	}
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return sub, false
	}
	return sub, true
}
