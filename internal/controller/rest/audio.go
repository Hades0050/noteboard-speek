package rest

import (
	"net/http"
	"path/filepath"
	"strings"
)

// audioExt — расширение по типу содержимого. Полагаться на имя файла из
// браузера нельзя: MediaRecorder отдаёт blob вообще без имени.
var audioExt = map[string]string{
	"audio/webm":  ".webm",
	"audio/ogg":   ".ogg",
	"audio/mp4":   ".m4a",
	"audio/mpeg":  ".mp3",
	"audio/wav":   ".wav",
	"audio/x-wav": ".wav",
}

func (s *Server) uploadAudio(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r)
	if err != nil {
		s.writeError(w, r, err)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, s.MaxAudioBytes)
	file, hdr, err := r.FormFile("audio")
	if err != nil {
		badRequest(w, "нужно поле audio: "+err.Error())
		return
	}
	defer file.Close()

	ext := audioExt[mimeOf(hdr.Header.Get("Content-Type"))]
	if ext == "" {
		if e := filepath.Ext(hdr.Filename); e != "" {
			ext = e
		} else {
			ext = ".webm"
		}
	}

	n, err := s.Notes.AttachAudio(id, file, ext)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, n)
}

func (s *Server) getAudio(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	n, err := s.Notes.Get(id)
	if err != nil {
		s.writeError(w, r, err)
		return
	}
	if n.AudioPath == "" {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, n.AudioPath)
}

// mimeOf отрезает параметры вида "; codecs=opus".
func mimeOf(header string) string {
	return strings.TrimSpace(strings.Split(header, ";")[0])
}
