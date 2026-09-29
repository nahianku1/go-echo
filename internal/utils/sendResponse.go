package utils

import (
	"github.com/labstack/echo/v5"
)

type ResponseData[T any] struct {
	Code    int    `json:"code"`
	Status  string `json:"status"`
	Message string `json:"msg"`
	Data    T      `json:"data"`
}

// SendResponse sends a standardized JSON response envelope
func SendResponse[T any](c *echo.Context, code int, message string, data T) error {
	status := "success"
	if code >= 400 {
		status = "error"
	}

	response := ResponseData[T]{
		Code:    code,
		Status:  status,
		Message: message,
		Data:    data,
	}

	return c.JSON(code, response)
}
