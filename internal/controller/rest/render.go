// Package rest — HTTP-транспорт для того, что плохо ложится на RPC: загрузка
// файлов, отдача аудио и картинок, health и статика фронта. Бизнес-логики
// здесь нет, она в usecase.
package rest

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"noteboard/internal/entity"
)

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError — единственное место, где ошибка превращается в статус.
// Добавился новый вид ошибки в домене — правится только этот switch.
func (s *Server) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, entity.ErrNotFound):
		writeJSON(w, http.StatusNotFound, errorBody{"not found"})
	case errors.Is(err, entity.ErrInvalid):
		writeJSON(w, http.StatusBadRequest, errorBody{err.Error()})
	default:
		s.Log.Error("внутренняя ошибка", "method", r.Method, "path", r.URL.Path, "err", err)
		writeJSON(w, http.StatusInternalServerError, errorBody{err.Error()})
	}
}

func badRequest(w http.ResponseWriter, msg string) {
	writeJSON(w, http.StatusBadRequest, errorBody{msg})
}

type errorBody struct {
	Error string `json:"error"`
}

// idParam читает {id} из пути.
func idParam(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, entity.Invalid("bad id")
	}
	return id, nil
}

func logAttrs(r *http.Request) []slog.Attr {
	return []slog.Attr{slog.String("method", r.Method), slog.String("path", r.URL.Path)}
}
