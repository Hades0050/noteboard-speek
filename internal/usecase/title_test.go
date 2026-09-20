package usecase

import (
	"strings"
	"testing"

	"noteboard/internal/entity"
)

func TestCardTitle(t *testing.T) {
	cases := []struct {
		name string
		note entity.Note
		want string
	}{
		{"заголовок как есть", entity.Note{Title: "План"}, "План"},
		{"из тела без тегов", entity.Note{Body: "<p>Купить <strong>хлеб</strong></p>"}, "Купить хлеб"},
		{"сущности разворачиваются", entity.Note{Body: "<p>a &amp; b</p>"}, "a & b"},
		{"из расшифровки", entity.Note{Transcript: "привет\nмир"}, "привет мир"},
		{"пустая заметка", entity.Note{}, "Заметка"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := CardTitle(c.note); got != c.want {
				t.Fatalf("CardTitle() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestCardTitleTruncates(t *testing.T) {
	long := strings.Repeat("я", 100)
	got := CardTitle(entity.Note{Body: "<p>" + long + "</p>"})
	if r := []rune(got); len(r) != titleRunes+1 || r[len(r)-1] != '…' {
		t.Fatalf("длина %d, хвост %q", len([]rune(got)), string([]rune(got)[len([]rune(got))-1]))
	}
}
