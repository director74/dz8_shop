package errors

import (
	"errors"
	"fmt"
	"log"
	"strings"
)

// Common errors
var (
	ErrNotFound           = errors.New("ресурс не найден")
	ErrAlreadyExists      = errors.New("ресурс уже существует")
	ErrInvalidCredentials = errors.New("неверные учетные данные")
	ErrUnauthorized       = errors.New("не авторизован")
	ErrForbidden          = errors.New("доступ запрещен")
	ErrInternalServer     = errors.New("внутренняя ошибка сервера")
	ErrBadRequest         = errors.New("некорректный запрос")
)

// AppendPrefix добавляет префикс к сообщению об ошибке
func AppendPrefix(err error, prefix string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", prefix, err)
}

// LogError логирует ошибку с контекстом
func LogError(err error, context string) {
	if err == nil {
		return
	}
	log.Printf("ОШИБКА [%s]: %v", context, err)
}

// LogErrorWithDetails логирует ошибку с контекстом и дополнительными деталями
func LogErrorWithDetails(err error, context string, details map[string]interface{}) {
	if err == nil {
		return
	}

	var detailsString strings.Builder
	for k, v := range details {
		if detailsString.Len() > 0 {
			detailsString.WriteString(", ")
		}
		detailsString.WriteString(fmt.Sprintf("%s=%v", k, v))
	}

	log.Printf("ОШИБКА [%s]: %v | Детали: %s", context, err, detailsString.String())
}

// ErrorGroup представляет группу ошибок, собранных из разных операций
type ErrorGroup struct {
	errors []error
}

// NewErrorGroup создает новую группу ошибок
func NewErrorGroup() *ErrorGroup {
	return &ErrorGroup{
		errors: make([]error, 0),
	}
}

// Add добавляет ошибку в группу (игнорирует nil)
func (g *ErrorGroup) Add(err error) {
	if err != nil {
		g.errors = append(g.errors, err)
	}
}

// AddPrefix добавляет ошибку с префиксом в группу
func (g *ErrorGroup) AddPrefix(err error, prefix string) {
	if err != nil {
		g.errors = append(g.errors, AppendPrefix(err, prefix))
	}
}

// HasErrors проверяет, есть ли ошибки в группе
func (g *ErrorGroup) HasErrors() bool {
	return len(g.errors) > 0
}

// Error возвращает конкатенацию всех ошибок в группе
func (g *ErrorGroup) Error() string {
	var sb strings.Builder
	for i, err := range g.errors {
		if i > 0 {
			sb.WriteString("; ")
		}
		sb.WriteString(err.Error())
	}
	return sb.String()
}

// ErrorWithDetails представляет ошибку с дополнительными деталями
type ErrorWithDetails struct {
	Err     error
	Details map[string]interface{}
}

// NewErrorWithDetails создает новую ошибку с деталями
func NewErrorWithDetails(err error, details map[string]interface{}) *ErrorWithDetails {
	return &ErrorWithDetails{
		Err:     err,
		Details: details,
	}
}

// Error реализует интерфейс error
func (e *ErrorWithDetails) Error() string {
	var sb strings.Builder
	sb.WriteString(e.Err.Error())

	if len(e.Details) > 0 {
		sb.WriteString(" (")
		first := true
		for k, v := range e.Details {
			if !first {
				sb.WriteString(", ")
			}
			sb.WriteString(fmt.Sprintf("%s=%v", k, v))
			first = false
		}
		sb.WriteString(")")
	}

	return sb.String()
}

// Unwrap возвращает оригинальную ошибку
func (e *ErrorWithDetails) Unwrap() error {
	return e.Err
}

// Is проверяет, соответствует ли ошибка target
func (e *ErrorWithDetails) Is(target error) bool {
	return errors.Is(e.Err, target)
}
