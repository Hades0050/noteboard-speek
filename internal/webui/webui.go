// Package webui отдаёт собранный Nuxt SPA, вшитый в бинарник.
package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
)

//go:embed all:dist
var dist embed.FS

// Handler отдаёт SPA и на неизвестный путь возвращает index.html, иначе
// клиентские роуты не переживают перезагрузку страницы. Непустой dir
// подменяет вшитую копию папкой на диске — удобно при разработке.
func Handler(dir string) (http.Handler, bool) {
	var root fs.FS
	if dir != "" {
		root = os.DirFS(dir)
	} else {
		sub, err := fs.Sub(dist, "dist")
		if err != nil {
			return nil, false
		}
		root = sub
	}
	if _, err := fs.Stat(root, "index.html"); err != nil {
		return nil, false
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}
		if st, err := fs.Stat(root, name); err != nil || st.IsDir() {
			if st2, err2 := fs.Stat(root, path.Join(name, "index.html")); err2 == nil && !st2.IsDir() {
				name = path.Join(name, "index.html")
			} else {
				name = "index.html"
			}
		}
		// Nuxt кладёт хеш содержимого в имя каждого файла под /_nuxt/.
		if strings.HasPrefix(name, "_nuxt/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		http.ServeFileFS(w, r, root, name)
	}), true
}
