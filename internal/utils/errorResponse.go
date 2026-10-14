package utils

import (
	"net/http"

	"github.com/labstack/echo/v5"
)

// ErrorResponse is retained for reference/documentation of the wire format; the
// actual rendering lives in middlewares.HTTPErrorHandler.
// type ErrorResponse struct {
// 	Code    int    `json:"code"`
// 	Status  string `json:"status"`
// 	Message string `json:"message"`
// }

// Err is the single helper for returning HTTP errors from handlers. It wraps
// echo.NewHTTPError so every error response keeps the standard shape:
//
//	return utils.Err(http.StatusUnauthorized, "Authentication token missing")
func Err(statusCode int, message string) error {
	return echo.NewHTTPError(statusCode, message)
}

// ErrInternal wraps an unexpected error as a 500 while preserving the cause for
// logging (echo's error handler logs it and hides it from the client):
//
//	return utils.ErrInternal("users could not be listed", err)
func ErrInternal(message string, err error) error {
	return echo.NewHTTPError(http.StatusInternalServerError, message).Wrap(err)
}
