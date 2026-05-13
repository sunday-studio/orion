package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed index.html assets
var files embed.FS

func Assets() http.FileSystem {
	assets, err := fs.Sub(files, "assets")
	if err != nil {
		return http.FS(files)
	}
	return http.FS(assets)
}

func Index() ([]byte, error) {
	return files.ReadFile("index.html")
}
