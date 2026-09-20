package rest

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// requestLogger пишет по строке на запрос в структурированный лог.
// Свой вместо middleware.Logger из chi: тот печатает цветной текст, а нам
// нужен slog, чтобы строки одинаково читались и в терминале, и в docker logs.
func (s *Server) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()

		next.ServeHTTP(ww, r)

		// Отдача фронта и статики забивала бы лог, оставляем только API.
		if len(r.URL.Path) < 4 || r.URL.Path[:4] != "/api" {
			return
		}
		s.Log.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"bytes", ww.BytesWritten(),
			"ms", time.Since(start).Milliseconds(),
			"req", middleware.GetReqID(r.Context()),
		)
	})
}
