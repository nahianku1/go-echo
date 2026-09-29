package utils

import (
	"net/http"

	"github.com/labstack/echo/v5"
)

func SetCookie(c *echo.Context, name string, value string, maxAge int) {
	c.SetCookie(&http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	})
}
