package usecase

import (
	"html"
	"regexp"
	"strings"

	"noteboard/internal/entity"
)

// titleRunes — сколько символов заголовка помещается на карточке.
const titleRunes = 60

var tagRE = regexp.MustCompile(`<[^>]*>`)

// CardTitle выбирает подпись для карточки: заголовок заметки, иначе начало
// текста, иначе начало расшифровки. Тело хранится как HTML из редактора,
// поэтому теги и сущности сначала убираются.
func CardTitle(n entity.Note) string {
	for _, candidate := range []string{n.Title, excerpt(n.Body), excerpt(n.Transcript)} {
		if candidate != "" {
			return candidate
		}
	}
	return "Заметка"
}

func excerpt(s string) string {
	s = tagRE.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > titleRunes {
		return strings.TrimSpace(string(r[:titleRunes])) + "…"
	}
	return s
}
