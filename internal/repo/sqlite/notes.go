package sqlite

import (
	"database/sql"
	"errors"
	"strings"

	"noteboard/internal/entity"
)

// NoteRepo — операции с заметками. Отдельный тип, а не метод на DB, чтобы
// сервисный слой зависел от узкого интерфейса, а не от всей базы.
type NoteRepo struct{ db *DB }

func NewNoteRepo(db *DB) *NoteRepo { return &NoteRepo{db: db} }

const noteColumns = `id, title, body, audio_path, transcript, transcribe_status,
	transcribe_error, pinned, created_at, updated_at`

type scanner interface{ Scan(dest ...any) error }

func scanNote(sc scanner) (entity.Note, error) {
	var (
		n      entity.Note
		status string
	)
	err := sc.Scan(&n.ID, &n.Title, &n.Body, &n.AudioPath, &n.Transcript, &status,
		&n.Error, &n.Pinned, &n.CreatedAt, &n.UpdatedAt)
	n.Status = entity.Status(status)
	return n, err
}

// List возвращает заметки, закреплённые сверху. Пустой q — весь список.
func (r *NoteRepo) List(q string) ([]entity.Note, error) {
	query := `SELECT ` + noteColumns + ` FROM notes`
	args := []any{}
	if q = strings.TrimSpace(q); q != "" {
		query += ` WHERE title LIKE ?1 OR body LIKE ?1 OR transcript LIKE ?1`
		args = append(args, "%"+q+"%")
	}
	query += ` ORDER BY pinned DESC, updated_at DESC`

	rows, err := r.db.sql.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []entity.Note{}
	for rows.Next() {
		n, err := scanNote(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (r *NoteRepo) Get(id int64) (entity.Note, error) {
	n, err := scanNote(r.db.sql.QueryRow(`SELECT `+noteColumns+` FROM notes WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return entity.Note{}, entity.ErrNotFound
	}
	return n, err
}

func (r *NoteRepo) Create(title, body string) (entity.Note, error) {
	ts := now()
	res, err := r.db.sql.Exec(`INSERT INTO notes(title, body, created_at, updated_at) VALUES(?,?,?,?)`,
		title, body, ts, ts)
	if err != nil {
		return entity.Note{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return entity.Note{}, err
	}
	return r.Get(id)
}

// Update пишет только те поля, которые есть в патче.
func (r *NoteRepo) Update(id int64, p entity.NotePatch) (entity.Note, error) {
	if p.Empty() {
		return r.Get(id)
	}
	sets := []string{}
	args := []any{}
	if p.Title != nil {
		sets, args = append(sets, "title = ?"), append(args, *p.Title)
	}
	if p.Body != nil {
		sets, args = append(sets, "body = ?"), append(args, *p.Body)
	}
	if p.Transcript != nil {
		sets, args = append(sets, "transcript = ?"), append(args, *p.Transcript)
	}
	if p.Pinned != nil {
		sets, args = append(sets, "pinned = ?"), append(args, *p.Pinned)
	}
	sets = append(sets, "updated_at = ?")
	args = append(args, now(), id)

	res, err := r.db.sql.Exec(`UPDATE notes SET `+strings.Join(sets, ", ")+` WHERE id = ?`, args...)
	if err != nil {
		return entity.Note{}, err
	}
	if affected(res) == 0 {
		return entity.Note{}, entity.ErrNotFound
	}
	return r.Get(id)
}

// Delete удаляет заметку вместе с её карточками на доске: карточка без
// заметки — пустая строка, которую всё равно пришлось бы убирать руками.
func (r *NoteRepo) Delete(id int64) error {
	tx, err := r.db.sql.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM cards WHERE note_id = ?`, id); err != nil {
		return err
	}
	res, err := tx.Exec(`DELETE FROM notes WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if affected(res) == 0 {
		return entity.ErrNotFound
	}
	return tx.Commit()
}

// AttachAudio привязывает файл к заметке и ставит её в очередь расшифровки.
func (r *NoteRepo) AttachAudio(id int64, path string) (entity.Note, error) {
	res, err := r.db.sql.Exec(`UPDATE notes SET audio_path = ?, transcribe_status = ?,
		transcribe_error = '', updated_at = ? WHERE id = ?`,
		path, entity.StatusPending, now(), id)
	if err != nil {
		return entity.Note{}, err
	}
	if affected(res) == 0 {
		return entity.Note{}, entity.ErrNotFound
	}
	return r.Get(id)
}

func affected(res sql.Result) int64 {
	n, err := res.RowsAffected()
	if err != nil {
		return 0
	}
	return n
}
