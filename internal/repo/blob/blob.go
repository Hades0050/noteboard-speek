// Package blob хранит бинарные файлы (аудио и картинки) в каталоге на диске.
// Интерфейс намеренно узкий: если однажды понадобится S3, подменяется он один.
package blob

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type Store struct{ dir string }

func New(dir string) *Store { return &Store{dir: dir} }

func (s *Store) Dir() string { return s.dir }

// Save пишет поток в файл вида <prefix>-<наносекунды><ext> и возвращает путь.
// Имя генерится сервером: пользовательское имя файла до диска не доходит.
func (s *Store) Save(prefix, ext string, src io.Reader) (string, error) {
	name := fmt.Sprintf("%s-%d%s", prefix, time.Now().UnixNano(), ext)
	path := filepath.Join(s.dir, name)

	out, err := os.Create(path)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(out, src); err != nil {
		out.Close()
		os.Remove(path)
		return "", err
	}
	if err := out.Close(); err != nil {
		os.Remove(path)
		return "", err
	}
	return path, nil
}

// Remove удаляет файл и молчит, если его уже нет.
func (s *Store) Remove(path string) {
	if path == "" {
		return
	}
	_ = os.Remove(path)
}

// Path собирает путь внутри каталога, отбрасывая любые ../ из имени.
func (s *Store) Path(name string) (string, bool) {
	clean := filepath.Base(name)
	if clean == "." || clean == string(filepath.Separator) || clean == ".." {
		return "", false
	}
	return filepath.Join(s.dir, clean), true
}
