package httpapi

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed web/*
var webFiles embed.FS

func webAssets() http.Handler {
	sub, _ := fs.Sub(webFiles, "web")
	return http.FileServer(http.FS(sub))
}
