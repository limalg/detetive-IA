package web

import (
	"embed"
	"io/fs"
	"net/http"
)

// Assets embeds all static files and html templates for the monolithic web server.
//
//go:embed static/* templates/*
var Assets embed.FS

// FileServer returns an http.Handler that serves static assets under /static/
// and index.html at root /.
func FileServer() http.Handler {
	mux := http.NewServeMux()

	staticFS, err := fs.Sub(Assets, "static")
	if err == nil {
		mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		data, err := Assets.ReadFile("templates/index.html")
		if err != nil {
			http.Error(w, "template not found", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
	})

	return mux
}
