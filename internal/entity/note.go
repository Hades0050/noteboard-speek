// Package entity содержит типы предметной области и правила, которые не зависят
// ни от базы, ни от HTTP. Сюда можно смотреть, чтобы понять, из чего состоит
// приложение, не читая инфраструктуру.
package entity

// Status — состояние расшифровки заметки.
type Status string

const (
	StatusNone    Status = "none"    // аудио нет
	StatusPending Status = "pending" // в очереди
	StatusRunning Status = "running" // воркер занят этой заметкой
	StatusDone    Status = "done"
	StatusError   Status = "error"
)

// Note — заметка. JSON-теги живут прямо здесь: в проекте такого размера
// отдельные DTO на каждый слой дали бы вдвое больше кода и ноль пользы.
type Note struct {
	ID         int64  `json:"id"`
	Title      string `json:"title"`
	Body       string `json:"body"` // HTML из редактора
	AudioPath  string `json:"audio_path"`
	Transcript string `json:"transcript"`
	Status     Status `json:"transcribe_status"`
	Error      string `json:"transcribe_error"`
	Pinned     bool   `json:"pinned"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// NotePatch — частичное обновление: nil-поле означает «не трогать».
// Явная структура вместо map[string]any убирает из репозитория проверку
// «а можно ли писать в эту колонку».
type NotePatch struct {
	Title      *string `json:"title"`
	Body       *string `json:"body"`
	Transcript *string `json:"transcript"`
	Pinned     *bool   `json:"pinned"`
}

func (p NotePatch) Empty() bool {
	return p.Title == nil && p.Body == nil && p.Transcript == nil && p.Pinned == nil
}
