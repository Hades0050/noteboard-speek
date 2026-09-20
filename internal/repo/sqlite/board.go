package sqlite

import (
	"database/sql"
	"strings"

	"noteboard/internal/entity"
)

// BoardRepo — колонки и карточки канбана.
type BoardRepo struct{ db *DB }

func NewBoardRepo(db *DB) *BoardRepo { return &BoardRepo{db: db} }

// Board отдаёт доску целиком: два запроса вместо N+1 по колонкам.
func (r *BoardRepo) Board() ([]entity.Column, error) {
	rows, err := r.db.sql.Query(`SELECT id, title, position FROM columns ORDER BY position`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols := []entity.Column{}
	index := map[int64]int{} // id колонки -> её место в срезе
	for rows.Next() {
		var c entity.Column
		if err := rows.Scan(&c.ID, &c.Title, &c.Position); err != nil {
			return nil, err
		}
		c.Cards = []entity.Card{}
		index[c.ID] = len(cols)
		cols = append(cols, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	cards, err := r.db.sql.Query(`SELECT c.id, c.column_id, c.title, c.body, c.position, c.note_id,
		COALESCE(n.title, ''), COALESCE(n.transcribe_status, ''), c.created_at, c.updated_at
		FROM cards c LEFT JOIN notes n ON n.id = c.note_id
		ORDER BY c.position`)
	if err != nil {
		return nil, err
	}
	defer cards.Close()

	for cards.Next() {
		var (
			c      entity.Card
			status string
		)
		if err := cards.Scan(&c.ID, &c.ColumnID, &c.Title, &c.Body, &c.Position, &c.NoteID,
			&c.NoteTitle, &status, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		c.NoteStatus = entity.Status(status)
		if i, ok := index[c.ColumnID]; ok {
			cols[i].Cards = append(cols[i].Cards, c)
		}
	}
	return cols, cards.Err()
}

func (r *BoardRepo) CreateColumn(title string) (entity.Column, error) {
	pos, err := r.nextPosition(`SELECT MAX(position) FROM columns`)
	if err != nil {
		return entity.Column{}, err
	}
	res, err := r.db.sql.Exec(`INSERT INTO columns(title, position) VALUES(?, ?)`, title, pos)
	if err != nil {
		return entity.Column{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return entity.Column{}, err
	}
	return entity.Column{ID: id, Title: title, Position: pos, Cards: []entity.Card{}}, nil
}

func (r *BoardRepo) UpdateColumn(id int64, title string) error {
	res, err := r.db.sql.Exec(`UPDATE columns SET title = ? WHERE id = ?`, title, id)
	if err != nil {
		return err
	}
	if affected(res) == 0 {
		return entity.ErrNotFound
	}
	return nil
}

func (r *BoardRepo) DeleteColumn(id int64) error {
	res, err := r.db.sql.Exec(`DELETE FROM columns WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if affected(res) == 0 {
		return entity.ErrNotFound
	}
	return nil
}

// CreateCard кладёт карточку в конец колонки. noteID может быть nil —
// это обычная карточка без связи с заметкой.
func (r *BoardRepo) CreateCard(columnID int64, title, body string, noteID *int64) (entity.Card, error) {
	pos, err := r.nextPosition(`SELECT MAX(position) FROM cards WHERE column_id = ?`, columnID)
	if err != nil {
		return entity.Card{}, err
	}
	ts := now()
	res, err := r.db.sql.Exec(`INSERT INTO cards(column_id, title, body, position, note_id, created_at, updated_at)
		VALUES(?,?,?,?,?,?,?)`, columnID, title, body, pos, noteID, ts, ts)
	if err != nil {
		return entity.Card{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return entity.Card{}, err
	}
	return entity.Card{ID: id, ColumnID: columnID, Title: title, Body: body, Position: pos,
		NoteID: noteID, CreatedAt: ts, UpdatedAt: ts}, nil
}

func (r *BoardRepo) UpdateCard(id int64, p entity.CardPatch) error {
	if p.Empty() {
		return nil
	}
	sets := []string{}
	args := []any{}
	if p.Title != nil {
		sets, args = append(sets, "title = ?"), append(args, *p.Title)
	}
	if p.Body != nil {
		sets, args = append(sets, "body = ?"), append(args, *p.Body)
	}
	if p.NoteID != nil {
		sets, args = append(sets, "note_id = ?"), append(args, *p.NoteID)
	}
	sets = append(sets, "updated_at = ?")
	args = append(args, now(), id)

	res, err := r.db.sql.Exec(`UPDATE cards SET `+strings.Join(sets, ", ")+` WHERE id = ?`, args...)
	if err != nil {
		return err
	}
	if affected(res) == 0 {
		return entity.ErrNotFound
	}
	return nil
}

func (r *BoardRepo) DeleteCard(id int64) error {
	res, err := r.db.sql.Exec(`DELETE FROM cards WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if affected(res) == 0 {
		return entity.ErrNotFound
	}
	return nil
}

// Reorder переписывает раскладку целых колонок одной транзакцией: перетаскивание
// не должно оставлять доску в половинчатом состоянии.
func (r *BoardRepo) Reorder(orders []entity.ColumnOrder) error {
	tx, err := r.db.sql.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // после Commit это no-op

	ts := now()
	stmt, err := tx.Prepare(`UPDATE cards SET column_id = ?, position = ?, updated_at = ? WHERE id = ?`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, o := range orders {
		for i, id := range o.CardIDs {
			if _, err := stmt.Exec(o.ColumnID, float64(i+1)*entity.PositionStep, ts, id); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

// nextPosition — конец списка: максимум плюс шаг.
func (r *BoardRepo) nextPosition(query string, args ...any) (float64, error) {
	var max sql.NullFloat64
	if err := r.db.sql.QueryRow(query, args...).Scan(&max); err != nil {
		return 0, err
	}
	return max.Float64 + entity.PositionStep, nil
}
