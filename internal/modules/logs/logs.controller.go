package logs

import (
	"net-http/internal/modules/logs/dto"
	"net-http/internal/utils"

	"github.com/labstack/echo/v5"
)

// Controller is the HTTP layer for the logs module.
//
// The use-case service is injected through NewController instead of being the
// package-level LogsService singleton, so the handler can be exercised with a
// stub service and the module holds no mutable global state.
type Controller struct {
	service *Service
}

// NewController injects the logs service.
func NewController(service *Service) *Controller {
	return &Controller{service: service}
}

func (c *Controller) GetAllLogs(ctx *echo.Context) error {
	// Generic query helpers: type-safe parsing with defaults, no strconv noise.
	page, _ := echo.QueryParamOr[int](ctx, "page", 1)
	limit, _ := echo.QueryParamOr[int](ctx, "limit", 20)
	statusCode, _ := echo.QueryParamOr[int](ctx, "status", 0)
	search, _ := echo.QueryParamOr[string](ctx, "search", "")
	method, _ := echo.QueryParamOr[string](ctx, "method", "")
	user, _ := echo.QueryParamOr[string](ctx, "user", "")

	res, err := c.service.GetAllLogs(ctx.Request().Context(), page, limit, search, method, user, statusCode)
	if err != nil {
		return utils.ErrInternal("could not list logs", err)
	}

	return utils.SendResponse[*dto.PaginatedLogsResponse](ctx, 200, "Logs retrieved successfully!", res)
}
