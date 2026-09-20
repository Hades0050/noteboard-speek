package stt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"noteboard/internal/config"
)

// openAIEngine работает с любым OpenAI-совместимым /v1/audio/transcriptions:
// сам OpenAI, speaches, faster-whisper-server, whisper.cpp в режиме сервера.
type openAIEngine struct {
	cfg    config.Transcribe
	client *http.Client
}

func newOpenAIEngine(cfg config.Transcribe) *openAIEngine {
	return &openAIEngine{
		cfg: cfg,
		// Свой клиент, а не http.DefaultClient: таймаут и пул соединений
		// должны принадлежать этому движку.
		client: &http.Client{Timeout: cfg.Timeout + time.Minute},
	}
}

func (e *openAIEngine) Name() string { return "api" }

func (e *openAIEngine) Transcribe(ctx context.Context, path string) (string, error) {
	body, contentType, err := e.form(path)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.cfg.APIURL, body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", contentType)
	if e.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.cfg.APIKey)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var out struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("неожиданный ответ API: %w", err)
	}
	return strings.TrimSpace(out.Text), nil
}

// form собирает multipart-запрос. Поля prompt, hotwords и vad_filter
// добавляются только если заданы: OpenAI про два последних не знает.
func (e *openAIEngine) form(path string) (io.Reader, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	part, err := mw.CreateFormFile("file", filepath.Base(path))
	if err != nil {
		return nil, "", err
	}
	if _, err := io.Copy(part, f); err != nil {
		return nil, "", err
	}

	fields := map[string]string{
		"model":           e.cfg.Model,
		"language":        e.cfg.Language,
		"prompt":          e.cfg.Prompt,
		"hotwords":        e.cfg.Hotwords,
		"vad_filter":      e.cfg.VAD,
		"response_format": "json",
	}
	for k, v := range fields {
		if v == "" {
			continue
		}
		if err := mw.WriteField(k, v); err != nil {
			return nil, "", err
		}
	}
	if err := mw.Close(); err != nil {
		return nil, "", err
	}
	return &buf, mw.FormDataContentType(), nil
}
