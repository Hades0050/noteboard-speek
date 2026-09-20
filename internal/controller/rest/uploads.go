package rest

import (
	"net/http"
	"path/filepath"

	"github.com/go-chi/chi/v5"
)

// imageExt — какие картинки принимаем из редактора и с каким расширением
// кладём на диск. Всё остальное отбиваем на входе.
var imageExt = map[string]string{
	"image/png":     ".png",
	"image/jpeg":    ".jpg",
	"image/gif":     ".gif",
	"image/webp":    ".webp",
	"image/avif":    ".avif",
	"image/svg+xml": ".svg",
}

func (s *Server) uploadImage(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, s.MaxImageBytes)
	file, hdr, err := r.FormFile("file")
	if err != nil {
		badRequest(w, "нужно поле file: "+err.Error())
		return
	}
	defer file.Close()

	ext := imageExt[mimeOf(hdr.Header.Get("Content-Type"))]
	if ext == "" {
		badRequest(w, "поддерживаются только картинки: png, jpeg, gif, webp, avif, svg")
		return
	}

	path, err := s.Uploads.Save("img", ext, file)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"url": "/api/uploads/" + baseName(path)})
}

func (s *Server) getUpload(w http.ResponseWriter, r *http.Request) {
	// Имя генерит сервер, но в запросе оно приходит от клиента — чистим путь.
	path, ok := s.Uploads.Path(chi.URLParam(r, "name"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	// Имя содержит наносекунды, файл никогда не меняется — можно кэшировать вечно.
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeFile(w, r, path)
}

func baseName(path string) string { return filepath.Base(path) }
