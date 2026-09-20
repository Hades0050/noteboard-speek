package rpc_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"

	"noteboard/internal/controller/rpc"
	pb "noteboard/internal/gen/noteboard/v1"
	"noteboard/internal/gen/noteboard/v1/noteboardv1connect"
	"noteboard/internal/repo/blob"
	"noteboard/internal/repo/sqlite"
	"noteboard/internal/usecase"
)

// noWake — заглушка воркера: в тесте расшифровку никто не будит.
type noWake struct{}

func (noWake) Notify() {}

// newServer поднимает настоящий стек (SQLite + сценарии + Connect) на
// httptest. Моки не нужны: база файловая и живёт в t.TempDir().
func newServer(t *testing.T) noteboardv1connect.NotesServiceClient {
	t.Helper()
	dir := t.TempDir()
	db, err := sqlite.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	notes := usecase.NewNotes(sqlite.NewNoteRepo(db), blob.New(dir), noWake{})
	board := usecase.NewBoard(sqlite.NewBoardRepo(db), sqlite.NewNoteRepo(db))

	mux := http.NewServeMux()
	rpc.Mount(mux, "/api/rpc", notes, board)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return noteboardv1connect.NewNotesServiceClient(srv.Client(), srv.URL+"/api/rpc")
}

func TestNotesServiceRoundTrip(t *testing.T) {
	client := newServer(t)
	ctx := context.Background()

	created, err := client.CreateNote(ctx, connect.NewRequest(&pb.CreateNoteRequest{Title: "Заметка", Body: "<p>тело</p>"}))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	id := created.Msg.GetNote().GetId()

	title := "Другая"
	updated, err := client.UpdateNote(ctx, connect.NewRequest(&pb.UpdateNoteRequest{Id: id, Title: &title}))
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if got := updated.Msg.GetNote(); got.GetTitle() != title || got.GetBody() != "<p>тело</p>" {
		t.Fatalf("патч задел лишнее: %+v", got)
	}

	// Регистр важен: LIKE в SQLite складывает регистры только для ASCII.
	list, err := client.ListNotes(ctx, connect.NewRequest(&pb.ListNotesRequest{Query: "Друг"}))
	if err != nil || len(list.Msg.GetNotes()) != 1 {
		t.Fatalf("поиск: %v, %v", list, err)
	}
}

// Ошибки предметной области должны приезжать кодами Connect, а не 500.
func TestNotesServiceNotFound(t *testing.T) {
	_, err := newServer(t).GetNote(context.Background(), connect.NewRequest(&pb.GetNoteRequest{Id: 404}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("ждали CodeNotFound, получили %v (%v)", connect.CodeOf(err), err)
	}
}
