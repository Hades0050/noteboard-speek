package usecase

// Сценарии работы с заметками: что происходит вокруг базы — файлы на диске,
// очередь расшифровки, чистка за удалённой заметкой.

import (
	"io"
	"strconv"

	"noteboard/internal/entity"
)

type NotesUseCase struct {
	repo  NoteRepo
	files FileStore
	queue Notifier
}

func NewNotes(repo NoteRepo, files FileStore, queue Notifier) *NotesUseCase {
	return &NotesUseCase{repo: repo, files: files, queue: queue}
}

func (s *NotesUseCase) List(q string) ([]entity.Note, error) { return s.repo.List(q) }
func (s *NotesUseCase) Get(id int64) (entity.Note, error)    { return s.repo.Get(id) }

func (s *NotesUseCase) Create(title, body string) (entity.Note, error) {
	return s.repo.Create(title, body)
}

func (s *NotesUseCase) Update(id int64, p entity.NotePatch) (entity.Note, error) {
	return s.repo.Update(id, p)
}

// Delete убирает заметку вместе с её аудиофайлом: иначе диск растёт молча.
func (s *NotesUseCase) Delete(id int64) error {
	n, err := s.repo.Get(id)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(id); err != nil {
		return err
	}
	s.files.Remove(n.AudioPath)
	return nil
}

// AttachAudio сохраняет запись, ставит заметку в очередь и будит воркер.
// Старый файл удаляется только после успешной записи нового.
func (s *NotesUseCase) AttachAudio(id int64, src io.Reader, ext string) (entity.Note, error) {
	old, err := s.repo.Get(id)
	if err != nil {
		return entity.Note{}, err
	}

	path, err := s.files.Save(prefix(id), ext, src)
	if err != nil {
		return entity.Note{}, err
	}

	n, err := s.repo.AttachAudio(id, path)
	if err != nil {
		s.files.Remove(path)
		return entity.Note{}, err
	}
	if old.AudioPath != path {
		s.files.Remove(old.AudioPath)
	}
	s.queue.Notify()
	return n, nil
}

// Retranscribe ставит в очередь уже загруженное аудио — например, после
// смены модели или подсказки.
func (s *NotesUseCase) Retranscribe(id int64) (entity.Note, error) {
	n, err := s.repo.Get(id)
	if err != nil {
		return entity.Note{}, err
	}
	if n.AudioPath == "" {
		return entity.Note{}, entity.Invalid("у заметки нет аудио")
	}
	n, err = s.repo.AttachAudio(id, n.AudioPath)
	if err != nil {
		return entity.Note{}, err
	}
	s.queue.Notify()
	return n, nil
}

func prefix(id int64) string { return "note-" + strconv.FormatInt(id, 10) }
