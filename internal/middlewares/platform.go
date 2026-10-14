package middlewares

import (
	"github.com/labstack/echo/v5"
	"strings"
)

const PlatformContextKey = "platform"

// Middleware returns the echo middleware.
func Platform() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			platform := strings.ToLower(strings.TrimSpace(c.Request().Header.Get("X-Platform")))

			c.Set(PlatformContextKey, platform)

			return next(c)
		}
	}
}
