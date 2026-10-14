package utils

import (
	"net-http/internal/config"
	"net/http"

	"github.com/labstack/echo/v5"
)

func SetCookie(c *echo.Context, name string, value string, maxAge int) {
	c.SetCookie(&http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   config.AppConfig.NODE_ENV == "production",
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	})
}

func SetRefreshTokenCookie(c *echo.Context, value string, maxAge int) {
	c.SetCookie(&http.Cookie{
		Name:     "refresh_token",
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   config.AppConfig.NODE_ENV == "production",
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
	})
}
