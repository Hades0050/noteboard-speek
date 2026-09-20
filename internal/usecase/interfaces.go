// Package usecase — слой сценариев (Use Cases в терминах Clean Architecture).
// Здесь живёт вся логика приложения и, главное, интерфейсы к внешнему миру:
// базе, файлам, распознаванию речи. Реализации лежат снаружи (repo/, stt/),
// поэтому зависимости смотрят внутрь — сценарии не знают про SQL и HTTP.
package usecase

import (
	"context"
	"io"

	"noteboard/internal/entity"
)

// NoteRepo — хранилище заметок.
type NoteRepo interface {
	List(q string) ([]entity.Note, error)
	Get(id int64) (entity.Note, error)
	Create(title, body string) (entity.Note, error)
	Update(id int64, p entity.NotePatch) (entity.Note, error)
	Delete(id int64) error
	AttachAudio(id int64, path string) (entity.Note, error)
}

// BoardRepo — хранилище доски.
type BoardRepo interface {
	Board() ([]entity.Column, error)
	CreateColumn(title string) (entity.Column, error)
	UpdateColumn(id int64, title string) error
	DeleteColumn(id int64) error
	CreateCard(columnID int64, title, body string, noteID *int64) (entity.Card, error)
	UpdateCard(id int64, p entity.CardPatch) error
	DeleteCard(id int64) error
	Reorder(orders []entity.ColumnOrder) error
}

// FileStore — бинарные файлы: аудио и картинки.
type FileStore interface {
	Save(prefix, ext string, src io.Reader) (string, error)
	Remove(path string)
}

// TranscribeQueue — очередь расшифровки.
type TranscribeQueue interface {
	Claim() (entity.Note, error)
	Finish(id int64, text string, failure error) error
	ResetRunning() error
}

// Engine — один способ распознать речь (HTTP-сервис или локальный бинарник).
type Engine interface {
	Name() string
	Transcribe(ctx context.Context, path string) (string, error)
}

// Converter приводит запись к формату, который принимают движки.
type Converter interface {
	ToWAV(ctx context.Context, path string) (out string, cleanup func(), err error)
}

// Notifier будит фоновый воркер. Ровно один метод: больше сценарию заметок
// от воркера не нужно.
type Notifier interface{ Notify() }
