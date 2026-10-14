package utils

import (
	"net-http/internal/config"
	"net/http"

	"github.com/labstack/echo/v5"
)

func DeleteCookie(c *echo.Context, name string) {
	c.SetCookie(&http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   config.AppConfig.NODE_ENV == "production",
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func DeleteRefreshTokenCookie(c *echo.Context) {
	c.SetCookie(&http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/api/auth",
		HttpOnly: true,
		Secure:   config.AppConfig.NODE_ENV == "production",
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}
