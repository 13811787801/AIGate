package server

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	"aigate/web"
)

func consoleFileServer() http.Handler {
	sub, err := fs.Sub(web.StaticFS, "static")
	if err != nil {
		panic(err)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/v1/") || p == "/healthz" {
			http.NotFound(w, r)
			return
		}

		name := strings.TrimPrefix(p, "/")
		if name == "" || strings.HasSuffix(p, "/") {
			name = "index.html"
		}
		name = path.Clean(name)

		data, err := fs.ReadFile(sub, name)
		if err != nil {
			// SPA fallback
			data, err = fs.ReadFile(sub, "index.html")
			if err != nil {
				http.NotFound(w, r)
				return
			}
			name = "index.html"
		}

		w.Header().Set("Content-Type", contentTypeByName(name))
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(data)
	})
}

func contentTypeByName(name string) string {
	switch path.Ext(name) {
	case ".html":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js":
		return "application/javascript; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	case ".png":
		return "image/png"
	case ".ico":
		return "image/x-icon"
	default:
		return "application/octet-stream"
	}
}
