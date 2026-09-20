package sqlite

import (
	"errors"
	"path/filepath"
	"testing"

	"noteboard/internal/entity"
)

// newDB поднимает базу в t.TempDir(): миграции прогоняются те же, что в бою.
func newDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestNoteRepoCRUD(t *testing.T) {
	r := NewNoteRepo(newDB(t))

	n, err := r.Create("Первая", "<p>тело</p>")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if n.ID == 0 || n.Status != entity.StatusNone {
		t.Fatalf("создано криво: %+v", n)
	}

	title := "Вторая"
	upd, err := r.Update(n.ID, entity.NotePatch{Title: &title})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if upd.Title != title || upd.Body != n.Body {
		t.Fatalf("патч задел лишнее: %+v", upd)
	}

	got, err := r.Get(n.ID)
	if err != nil || got.Title != title {
		t.Fatalf("get: %+v, %v", got, err)
	}

	if err := r.Delete(n.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := r.Get(n.ID); !errors.Is(err, entity.ErrNotFound) {
		t.Fatalf("после удаления ждали ErrNotFound, получили %v", err)
	}
}

func TestNoteRepoListSearch(t *testing.T) {
	r := NewNoteRepo(newDB(t))
	if _, err := r.Create("Молоко", "купить"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Create("Работа", "созвон"); err != nil {
		t.Fatal(err)
	}

	all, err := r.List("")
	if err != nil || len(all) != 2 {
		t.Fatalf("список: %d, %v", len(all), err)
	}

	found, err := r.List("созв")
	if err != nil || len(found) != 1 || found[0].Title != "Работа" {
		t.Fatalf("поиск: %+v, %v", found, err)
	}
}

func TestQueueClaimFinish(t *testing.T) {
	db := newDB(t)
	notes, queue := NewNoteRepo(db), NewQueueRepo(db)

	n, err := notes.Create("голос", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := notes.AttachAudio(n.ID, "/tmp/voice.webm"); err != nil {
		t.Fatalf("attach: %v", err)
	}

	claimed, err := queue.Claim()
	if err != nil || claimed.ID != n.ID {
		t.Fatalf("claim: %+v, %v", claimed, err)
	}
	// Очередь пуста: заметка уже running, второй воркер её не подхватит.
	if _, err := queue.Claim(); !errors.Is(err, entity.ErrNotFound) {
		t.Fatalf("повторный claim: ждали ErrNotFound, получили %v", err)
	}

	if err := queue.Finish(n.ID, "распознанный текст", nil); err != nil {
		t.Fatalf("finish: %v", err)
	}
	done, err := notes.Get(n.ID)
	if err != nil {
		t.Fatal(err)
	}
	if done.Status != entity.StatusDone || done.Transcript != "распознанный текст" {
		t.Fatalf("после finish: %+v", done)
	}
}
