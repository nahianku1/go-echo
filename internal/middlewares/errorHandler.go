package middlewares

import (
	"errors"
	"github.com/labstack/echo/v5"
	"net-http/internal/utils"
	"net/http"
)

// errorEnvelope is the standard JSON error body for every failed response.
type errorEnvelope struct {
	Code    int    `json:"code"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// HTTPErrorHandler is the single place error bodies are rendered. Handlers,
// guards and the validator just return echo.NewHTTPError(...) (or any error);
// this handler turns it into the project's standard JSON envelope.
//
//	*echo.HTTPError        -> its Code + Message
//	echo sentinel errors   -> echo.StatusCode(err) + http.StatusText
//	anything else, or 5xx  -> 500 "Internal Server Error" (cause logged, not leaked)
func HTTPErrorHandler(c *echo.Context, err error) {
	code := echo.StatusCode(err)
	message := http.StatusText(code)

	var he *echo.HTTPError
	if errors.As(err, &he) {
		code = he.Code
		if he.Message != "" {
			message = he.Message
		}
	} else if code == 0 {
		code = http.StatusInternalServerError
	}

	if code >= 500 {
		utils.NewLogger("ERROR").Printf("handler error: %v", err)
		message = "Internal Server Error"
	}

	if r, _ := echo.UnwrapResponse(c.Response()); r != nil && r.Committed {
		return
	}

	_ = c.JSON(code, errorEnvelope{Code: code, Status: http.StatusText(code), Message: message})
}
