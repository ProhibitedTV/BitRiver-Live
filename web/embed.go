package web

import (
	"embed"
	"io/fs"
)

// staticFiles bundles the web control center assets.
//
//go:embed static/*
var staticFiles embed.FS

// Static returns a filesystem rooted at the bundled static assets.
func Static() (fs.FS, error) {
	return fs.Sub(staticFiles, "static")
}
