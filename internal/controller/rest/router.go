package rest

import (
	"io"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"noteboard/internal/entity"
	"noteboard/internal/repo/blob"
)

// NoteService — всё, что транспорту нужно от сценариев заметок. После
// переезда CRUD на ConnectRPC здесь осталась только работа с файлами.
type NoteService interface {
	Get(id int64) (entity.Note, error)
	AttachAudio(id int64, src io.Reader, ext string) (entity.Note, error)
}

// Deps — всё, что нужно транспорту. Явная структура вместо длинного списка
// аргументов: добавить зависимость можно, не переписывая вызовы.
type Deps struct {
	Notes         NoteService
	Uploads       *blob.Store
	Static        http.Handler // собранный фронт; nil — отдаём только /api
	Log           *slog.Logger
	Mode          func() string // режим расшифровки для /api/health
	MaxAudioBytes int64
	MaxImageBytes int64
}

type Server struct{ Deps }

// New собирает роутер. CRUD живёт в ConnectRPC (см. internal/controller/rpc);
// REST остался там, где RPC неудобен: загрузка и отдача файлов, health,
// статика фронта.
func New(d Deps) http.Handler {
	s := &Server{Deps: d}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(s.requestLogger)

	r.Route("/api", func(r chi.Router) {
		r.Get("/health", s.health)

		r.Post("/notes/{id}/audio", s.uploadAudio)
		r.Get("/notes/{id}/audio", s.getAudio)

		r.Post("/uploads", s.uploadImage)
		r.Get("/uploads/{name}", s.getUpload)
	})

	if s.Static != nil {
		r.Handle("/*", s.Static)
	}
	return r
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "transcribe": s.Mode()})
}
