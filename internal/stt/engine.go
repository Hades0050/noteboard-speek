// Package stt — адаптеры распознавания речи (Speech-To-Text).
//
// Движок скрыт за интерфейсом Engine: HTTP-сервис и локальный whisper.cpp
// отличаются только реализацией, а воркер об этом ничего не знает.
package stt

import (
	"context"

	"noteboard/internal/config"
)

// Engine — один способ распознать файл. На вход всегда путь к готовому WAV.
type Engine interface {
	Name() string
	Transcribe(ctx context.Context, path string) (string, error)
}

// NewEngine выбирает движок по конфигурации. nil означает, что расшифровка
// выключена — это нормальный режим работы, а не ошибка.
func NewEngine(cfg config.Transcribe) Engine {
	switch cfg.Mode() {
	case "api":
		return newOpenAIEngine(cfg)
	case "whisper.cpp":
		return newWhisperCLI(cfg)
	default:
		return nil
	}
}
