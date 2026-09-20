package web

import (
	"embed"
	"io/fs"
)

//go:embed dist/*
var distFS embed.FS

// GetStaticFS returns the embedded dist filesystem
func GetStaticFS() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}
