package errors

import (
	"errors"
	"fmt"
	"net/http"
)

// ServiceError представляет ошибку микросервиса с HTTP-статусом
type ServiceError struct {
	Code    int    // HTTP-статус
	Message string // Сообщение об ошибке
	Err     error  // Исходная ошибка
}

// NewServiceError создает новую ошибку сервиса
func NewServiceError(code int, message string, err error) *ServiceError {
	return &ServiceError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// Error реализует интерфейс error
func (e *ServiceError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// Unwrap возвращает оригинальную ошибку
func (e *ServiceError) Unwrap() error {
	return e.Err
}

func NewNotFoundError(resourceType string, id interface{}) *ServiceError {
	message := fmt.Sprintf("%s с ID=%v не найден", resourceType, id)
	return NewServiceError(http.StatusNotFound, message, ErrNotFound)
}

func NewAlreadyExistsError(resourceType string, field string, value interface{}) *ServiceError {
	message := fmt.Sprintf("%s с %s=%v уже существует", resourceType, field, value)
	return NewServiceError(http.StatusConflict, message, ErrAlreadyExists)
}

func NewInvalidCredentialsError() *ServiceError {
	return NewServiceError(http.StatusUnauthorized, "Неверное имя пользователя или пароль", ErrInvalidCredentials)
}

func NewUnauthorizedError(reason string) *ServiceError {
	message := "Требуется авторизация"
	if reason != "" {
		message = fmt.Sprintf("%s: %s", message, reason)
	}
	return NewServiceError(http.StatusUnauthorized, message, ErrUnauthorized)
}

func NewForbiddenError(reason string) *ServiceError {
	message := "Доступ запрещен"
	if reason != "" {
		message = fmt.Sprintf("%s: %s", message, reason)
	}
	return NewServiceError(http.StatusForbidden, message, ErrForbidden)
}

func NewInternalServerError(err error) *ServiceError {
	return NewServiceError(http.StatusInternalServerError, "Внутренняя ошибка сервера", err)
}

func NewBadRequestError(reason string) *ServiceError {
	message := "Некорректный запрос"
	if reason != "" {
		message = fmt.Sprintf("%s: %s", message, reason)
	}
	return NewServiceError(http.StatusBadRequest, message, ErrBadRequest)
}

func NewValidationError(field, reason string) *ServiceError {
	message := fmt.Sprintf("Ошибка валидации поля '%s': %s", field, reason)
	return NewServiceError(http.StatusBadRequest, message, ErrBadRequest)
}

// ToHTTPResponse преобразует ошибку в HTTP-ответ
func ToHTTPResponse(err error) (int, interface{}) {
	var se *ServiceError
	if errors.As(err, &se) {
		return se.Code, map[string]string{
			"error": se.Message,
		}
	}

	switch {
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound, map[string]string{
			"error": err.Error(),
		}
	case errors.Is(err, ErrAlreadyExists):
		return http.StatusConflict, map[string]string{
			"error": err.Error(),
		}
	case errors.Is(err, ErrInvalidCredentials), errors.Is(err, ErrUnauthorized):
		return http.StatusUnauthorized, map[string]string{
			"error": err.Error(),
		}
	case errors.Is(err, ErrForbidden):
		return http.StatusForbidden, map[string]string{
			"error": err.Error(),
		}
	case errors.Is(err, ErrBadRequest):
		return http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		}
	default:
		return http.StatusInternalServerError, map[string]string{
			"error": "Внутренняя ошибка сервера",
		}
	}
}

func HandleServiceError(err error, context string) *ServiceError {
	var se *ServiceError
	if errors.As(err, &se) {
		LogError(err, context)
		return se
	}

	LogError(err, context)

	switch {
	case errors.Is(err, ErrNotFound):
		return NewServiceError(http.StatusNotFound, err.Error(), err)
	case errors.Is(err, ErrAlreadyExists):
		return NewServiceError(http.StatusConflict, err.Error(), err)
	case errors.Is(err, ErrInvalidCredentials), errors.Is(err, ErrUnauthorized):
		return NewServiceError(http.StatusUnauthorized, err.Error(), err)
	case errors.Is(err, ErrForbidden):
		return NewServiceError(http.StatusForbidden, err.Error(), err)
	case errors.Is(err, ErrBadRequest):
		return NewServiceError(http.StatusBadRequest, err.Error(), err)
	default:
		return NewServiceError(http.StatusInternalServerError, "Внутренняя ошибка сервера", err)
	}
}
