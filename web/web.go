package web

import (
	"embed"
	"io/fs"
	"net/http"
)

// Assets embeds all static files, html templates, and banco3d assets for the monolithic web server.
//
//go:embed static/* templates/* banco3d/*
var Assets embed.FS

// FileServer returns an http.Handler that serves static assets under /static/,
// banco 3D under /banco/, and index.html at root /.
func FileServer() http.Handler {
	mux := http.NewServeMux()

	staticFS, err := fs.Sub(Assets, "static")
	if err == nil {
		mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	}

	bancoFS, err := fs.Sub(Assets, "banco3d")
	if err == nil {
		mux.Handle("/banco/", http.StripPrefix("/banco/", http.FileServer(http.FS(bancoFS))))
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
