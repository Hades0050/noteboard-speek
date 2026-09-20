package usecase

// Сценарии доски. Здесь же единственное настоящее правило предметной области:
// как из заметки получается заголовок карточки (см. title.go).

import (
	"strings"

	"noteboard/internal/entity"
)

type BoardUseCase struct {
	repo  BoardRepo
	notes NoteRepo
}

func NewBoard(repo BoardRepo, notes NoteRepo) *BoardUseCase {
	return &BoardUseCase{repo: repo, notes: notes}
}

func (s *BoardUseCase) Board() ([]entity.Column, error) { return s.repo.Board() }

func (s *BoardUseCase) CreateColumn(title string) (entity.Column, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return entity.Column{}, entity.Invalid("пустой title")
	}
	return s.repo.CreateColumn(title)
}

func (s *BoardUseCase) RenameColumn(id int64, title string) error {
	title = strings.TrimSpace(title)
	if title == "" {
		return entity.Invalid("пустой title")
	}
	return s.repo.UpdateColumn(id, title)
}

func (s *BoardUseCase) DeleteColumn(id int64) error { return s.repo.DeleteColumn(id) }

// CreateCard создаёт карточку; если она привязана к заметке и заголовок пуст,
// он берётся из самой заметки.
func (s *BoardUseCase) CreateCard(columnID int64, title, body string, noteID *int64) (entity.Card, error) {
	if columnID == 0 {
		return entity.Card{}, entity.Invalid("нужен column_id")
	}

	var note entity.Note
	if noteID != nil {
		n, err := s.notes.Get(*noteID)
		if err != nil {
			return entity.Card{}, err
		}
		note = n
		if strings.TrimSpace(title) == "" {
			title = CardTitle(n)
		}
	}

	card, err := s.repo.CreateCard(columnID, title, body, noteID)
	if err != nil {
		return entity.Card{}, err
	}
	if noteID != nil {
		card.NoteTitle, card.NoteStatus = note.Title, note.Status
	}
	return card, nil
}

// AttachNote кладёт заметку на доску. Без column_id она попадает в первую
// колонку — «На доску» в интерфейсе должно работать одним нажатием.
func (s *BoardUseCase) AttachNote(noteID, columnID int64) (entity.Card, error) {
	if columnID == 0 {
		cols, err := s.repo.Board()
		if err != nil {
			return entity.Card{}, err
		}
		if len(cols) == 0 {
			return entity.Card{}, entity.Invalid("на доске нет колонок")
		}
		columnID = cols[0].ID
	}
	return s.CreateCard(columnID, "", "", &noteID)
}

func (s *BoardUseCase) UpdateCard(id int64, p entity.CardPatch) error {
	return s.repo.UpdateCard(id, p)
}
func (s *BoardUseCase) DeleteCard(id int64) error { return s.repo.DeleteCard(id) }

func (s *BoardUseCase) Reorder(orders []entity.ColumnOrder) error {
	if len(orders) == 0 {
		return entity.Invalid("пустой columns")
	}
	return s.repo.Reorder(orders)
}
