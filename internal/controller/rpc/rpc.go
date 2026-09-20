// Package rpc — второй транспорт поверх тех же сценариев: ConnectRPC.
//
// Браузер не умеет чистый gRPC (нет доступа к HTTP/2-фреймам), поэтому взят
// Connect: тот же .proto, тот же путь /noteboard.v1.NotesService/Method, но
// поверх обычного HTTP/1.1 и без Envoy. gRPC-клиенты (grpcurl, Go) тоже
// работают — их формат Connect понимает на том же порту.
package rpc

import (
	"errors"
	"net/http"

	"connectrpc.com/connect"

	"noteboard/internal/entity"
	pb "noteboard/internal/gen/noteboard/v1"
	"noteboard/internal/gen/noteboard/v1/noteboardv1connect"
)

// Интерфейсы объявлены здесь же, у потребителя: транспорт описывает, что ему
// нужно от сценариев, а не наоборот.
type NoteService interface {
	List(q string) ([]entity.Note, error)
	Get(id int64) (entity.Note, error)
	Create(title, body string) (entity.Note, error)
	Update(id int64, p entity.NotePatch) (entity.Note, error)
	Delete(id int64) error
	Retranscribe(id int64) (entity.Note, error)
}

type BoardService interface {
	Board() ([]entity.Column, error)
	CreateColumn(title string) (entity.Column, error)
	RenameColumn(id int64, title string) error
	DeleteColumn(id int64) error
	CreateCard(columnID int64, title, body string, noteID *int64) (entity.Card, error)
	AttachNote(noteID, columnID int64) (entity.Card, error)
	UpdateCard(id int64, p entity.CardPatch) error
	DeleteCard(id int64) error
	Reorder(orders []entity.ColumnOrder) error
}

// Mount вешает оба сервиса на mux под префиксом prefix ("/api/rpc"):
// один префикс — одно правило в dev-прокси Nuxt и в reverse proxy.
func Mount(mux *http.ServeMux, prefix string, notes NoteService, board BoardService) {
	path, h := noteboardv1connect.NewNotesServiceHandler(&notesServer{notes})
	mux.Handle(prefix+path, http.StripPrefix(prefix, h))

	path, h = noteboardv1connect.NewBoardServiceHandler(&boardServer{board})
	mux.Handle(prefix+path, http.StripPrefix(prefix, h))
}

// errorOf переводит ошибки предметной области в коды Connect. Это единственное
// место, где транспорт знает про entity.ErrNotFound — сценарии кодов не видят.
func errorOf(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, entity.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, entity.ErrInvalid):
		return connect.NewError(connect.CodeInvalidArgument, err)
	default:
		return connect.NewError(connect.CodeInternal, err)
	}
}

func notePB(n entity.Note) *pb.Note {
	return &pb.Note{
		Id:               n.ID,
		Title:            n.Title,
		Body:             n.Body,
		AudioPath:        n.AudioPath,
		Transcript:       n.Transcript,
		TranscribeStatus: string(n.Status),
		TranscribeError:  n.Error,
		Pinned:           n.Pinned,
		CreatedAt:        n.CreatedAt,
		UpdatedAt:        n.UpdatedAt,
	}
}

func cardPB(c entity.Card) *pb.Card {
	return &pb.Card{
		Id:         c.ID,
		ColumnId:   c.ColumnID,
		Title:      c.Title,
		Body:       c.Body,
		Position:   c.Position,
		NoteId:     c.NoteID,
		NoteTitle:  c.NoteTitle,
		NoteStatus: string(c.NoteStatus),
		CreatedAt:  c.CreatedAt,
		UpdatedAt:  c.UpdatedAt,
	}
}

func columnPB(c entity.Column) *pb.Column {
	out := &pb.Column{Id: c.ID, Title: c.Title, Position: c.Position}
	for _, card := range c.Cards {
		out.Cards = append(out.Cards, cardPB(card))
	}
	return out
}
