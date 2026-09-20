package web

import (
	"embed"
	"io/fs"
)

//go:embed dist/*
var assets embed.FS

func StaticFiles() fs.FS {
	content, err := fs.Sub(assets, "dist")
	if err != nil {
		panic(err)
	}
	return content
}
