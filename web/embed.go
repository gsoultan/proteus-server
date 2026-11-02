// Copyright (c) 2026 Gembit Soultan Shirazi <gembit.soultan@gmail.com>

// Package web serves the template builder, a single-page app built with
// Vite into dist/ and embedded in the binary, so the API and the UI share
// an origin and no CORS is needed. Without a build (dist/ holds only
// .gitkeep) the handler explains how to build it.
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var dist embed.FS

// Handler serves the app: files from dist/ with long cache headers for the
// hashed assets, and index.html for every other path so client-side routes
// work on reload. API paths never reach it; they are mounted first.
func Handler() http.Handler {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	index, indexErr := fs.ReadFile(sub, "index.html")
	files := http.FileServerFS(sub)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only GET and HEAD serve the app; anything else at a non-RPC path
		// is not found, so the server keeps no REST-looking surface.
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}
		if indexErr != nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			http.Error(w, "proteus-server is running. The web UI was not built into this binary: run `bun install && bun run build` in web/ and rebuild, or use the API directly.", http.StatusNotFound)
			return
		}
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if p != "" && p != "index.html" {
			if f, err := sub.Open(p); err == nil {
				f.Close()
				if strings.HasPrefix(p, "assets/") {
					w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				} else {
					w.Header().Set("Cache-Control", "no-cache")
				}
				files.ServeHTTP(w, r)
				return
			}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodGet {
			_, _ = w.Write(index)
		}
	})
}

// Built reports whether a UI build is embedded.
func Built() bool {
	_, err := fs.Stat(dist, "dist/index.html")
	return err == nil
}
