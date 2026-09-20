package entity

// Column — колонка канбана вместе с карточками (доска отдаётся одним запросом).
type Column struct {
	ID       int64   `json:"id"`
	Title    string  `json:"title"`
	Position float64 `json:"position"`
	Cards    []Card  `json:"cards"`
}

// Card — карточка. NoteID связывает её с заметкой; NoteTitle и NoteStatus —
// копии для отрисовки, они только на чтение и в базе не хранятся.
type Card struct {
	ID         int64   `json:"id"`
	ColumnID   int64   `json:"column_id"`
	Title      string  `json:"title"`
	Body       string  `json:"body"`
	Position   float64 `json:"position"`
	NoteID     *int64  `json:"note_id"`
	NoteTitle  string  `json:"note_title,omitempty"`
	NoteStatus Status  `json:"note_status,omitempty"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`
}

// CardPatch — частичное обновление карточки.
type CardPatch struct {
	Title  *string `json:"title"`
	Body   *string `json:"body"`
	NoteID *int64  `json:"note_id"`
}

func (p CardPatch) Empty() bool { return p.Title == nil && p.Body == nil && p.NoteID == nil }

// ColumnOrder — порядок карточек в одной колонке после перетаскивания.
type ColumnOrder struct {
	ColumnID int64   `json:"column_id"`
	CardIDs  []int64 `json:"card_ids"`
}

// PositionStep — шаг между позициями. Дробные позиции позволяют вставлять
// карточку между соседями, не переписывая всю колонку.
const PositionStep = 1000
