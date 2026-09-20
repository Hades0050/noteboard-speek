package sqlite

import (
	"database/sql"
	"errors"

	"noteboard/internal/entity"
)

// QueueRepo — очередь расшифровки. Отдельной таблицы нет: состояние живёт в
// самой заметке, и этого хватает, пока воркер один.
type QueueRepo struct{ db *DB }

func NewQueueRepo(db *DB) *QueueRepo { return &QueueRepo{db: db} }

// Claim забирает одну заметку из очереди и помечает её running.
// Возвращает entity.ErrNotFound, когда очередь пуста.
func (r *QueueRepo) Claim() (entity.Note, error) {
	n, err := scanNote(r.db.sql.QueryRow(`SELECT `+noteColumns+` FROM notes
		WHERE transcribe_status = ? ORDER BY id LIMIT 1`, entity.StatusPending))
	if errors.Is(err, sql.ErrNoRows) {
		return entity.Note{}, entity.ErrNotFound
	}
	if err != nil {
		return entity.Note{}, err
	}
	if _, err := r.db.sql.Exec(`UPDATE notes SET transcribe_status = ? WHERE id = ?`,
		entity.StatusRunning, n.ID); err != nil {
		return entity.Note{}, err
	}
	n.Status = entity.StatusRunning
	return n, nil
}

// Finish сохраняет результат: текст или ошибку расшифровки.
func (r *QueueRepo) Finish(id int64, text string, failure error) error {
	if failure != nil {
		_, err := r.db.sql.Exec(`UPDATE notes SET transcribe_status = ?, transcribe_error = ?,
			updated_at = ? WHERE id = ?`, entity.StatusError, failure.Error(), now(), id)
		return err
	}
	_, err := r.db.sql.Exec(`UPDATE notes SET transcribe_status = ?, transcribe_error = '',
		transcript = ?, updated_at = ? WHERE id = ?`, entity.StatusDone, text, now(), id)
	return err
}

// ResetRunning возвращает в очередь заметки, которые застряли на падении
// процесса: без этого они навсегда остались бы в статусе running.
func (r *QueueRepo) ResetRunning() error {
	_, err := r.db.sql.Exec(`UPDATE notes SET transcribe_status = ? WHERE transcribe_status = ?`,
		entity.StatusPending, entity.StatusRunning)
	return err
}
