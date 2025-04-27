package errors

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// HTTPErrorResponse представляет структуру HTTP ответа об ошибке
type HTTPErrorResponse struct {
	Error   string      `json:"error"`
	Details interface{} `json:"details,omitempty"`
}

func ErrorResponse(message string, details interface{}) HTTPErrorResponse {
	return HTTPErrorResponse{
		Error:   message,
		Details: details,
	}
}

func ErrorMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Если есть ошибки после выполнения запроса
		if len(c.Errors) > 0 {
			err := c.Errors.Last().Err
			code, response := ToHTTPResponse(err)
			c.JSON(code, response)
			c.Abort()
			return
		}
	}
}

func HandleGinError(c *gin.Context, err error) bool {
	if err != nil {
		var se *ServiceError
		if errors.As(err, &se) {
			c.JSON(se.Code, ErrorResponse(se.Message, nil))
		} else {
			// Определяем код ошибки
			code := http.StatusInternalServerError
			message := "Внутренняя ошибка сервера"

			switch {
			case errors.Is(err, ErrNotFound):
				code = http.StatusNotFound
				message = err.Error()
			case errors.Is(err, ErrAlreadyExists):
				code = http.StatusConflict
				message = err.Error()
			case errors.Is(err, ErrInvalidCredentials), errors.Is(err, ErrUnauthorized):
				code = http.StatusUnauthorized
				message = err.Error()
			case errors.Is(err, ErrForbidden):
				code = http.StatusForbidden
				message = err.Error()
			case errors.Is(err, ErrBadRequest):
				code = http.StatusBadRequest
				message = err.Error()
			}

			c.JSON(code, ErrorResponse(message, nil))
		}
		c.Abort()
		return true
	}
	return false
}

// BindJSON привязывает JSON к структуре и обрабатывает ошибки
func BindJSON(c *gin.Context, obj interface{}) bool {
	if err := c.ShouldBindJSON(obj); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(
			fmt.Sprintf("Ошибка в JSON данных: %v", err), nil,
		))
		c.Abort()
		return false
	}
	return true
}

// BindQuery привязывает параметры запроса к структуре
func BindQuery(c *gin.Context, obj interface{}) bool {
	if err := c.ShouldBindQuery(obj); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse(
			fmt.Sprintf("Ошибка в параметрах запроса: %v", err), nil,
		))
		c.Abort()
		return false
	}
	return true
}

func NotFoundHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusNotFound, ErrorResponse(
			fmt.Sprintf("Путь не найден: %s", c.Request.URL.Path), nil,
		))
	}
}

func MethodNotAllowedHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusMethodNotAllowed, ErrorResponse(
			fmt.Sprintf("Метод %s не поддерживается для пути %s", c.Request.Method, c.Request.URL.Path), nil,
		))
	}
}

func RecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				var err error
				switch t := r.(type) {
				case string:
					err = fmt.Errorf("паника: %s", t)
				case error:
					err = fmt.Errorf("паника: %w", t)
				default:
					err = fmt.Errorf("паника: %v", r)
				}
				LogError(err, "Recovery")
				c.JSON(http.StatusInternalServerError, ErrorResponse("Внутренняя ошибка сервера", nil))
				c.Abort()
			}
		}()
		c.Next()
	}
}
