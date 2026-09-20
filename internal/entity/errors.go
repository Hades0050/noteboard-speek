package entity

import "errors"

// Ошибки домена. Транспорт переводит их в HTTP-коды и больше ничего не знает
// про то, откуда они пришли — из базы, из файловой системы или из правил.
var (
	ErrNotFound = errors.New("not found")
	ErrInvalid  = errors.New("invalid input")
)

// Invalid оборачивает ErrInvalid с понятным человеку текстом.
func Invalid(msg string) error { return invalidError{msg} }

type invalidError struct{ msg string }

func (e invalidError) Error() string { return e.msg }
func (e invalidError) Is(target error) bool {
	return target == ErrInvalid
}
