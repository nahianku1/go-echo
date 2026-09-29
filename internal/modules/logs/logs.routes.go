package logs

import (
	"net-http/internal/middlewares"

	"github.com/labstack/echo/v5"
)

// RegisterRoutes mounts the logs endpoints on the given echo instance.
//
// As in the auth module, registration is a method that receives its guard rather
// than referencing a global, keeping the wiring explicit and testable.
func (c *Controller) RegisterRoutes(e *echo.Echo, auth *middlewares.Auth) {
	// Protected by the injected auth middleware (Admin Only)
	e.GET("/api/logs", c.GetAllLogs, auth.Require("admin"))
}
