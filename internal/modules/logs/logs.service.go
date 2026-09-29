package logs

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"net-http/internal/middlewares"
	"net-http/internal/modules/logs/dto"
)

// Service reads the request-log table.
//
// The *gorm.DB handle is injected rather than fetched from the package-level
// database.DB global, so the query layer declares its storage dependency in its
// own signature and can be pointed at a test database without touching config.
type Service struct {
	db *gorm.DB
}

// NewService injects the persistence dependency.
func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

func toLogResponse(log *middlewares.RequestLog) dto.LogResponse {
	return dto.LogResponse{
		ID:         log.ID,
		CreatedAt:  log.CreatedAt,
		Method:     log.Method,
		Path:       log.Path,
		StatusCode: log.StatusCode,
		DurationMs: log.DurationMs,
		IP:         log.IP,
		OS:         log.OS,
		Browser:    log.Browser,
		UserAgent:  log.UserAgent,
		User:       log.User,
	}
}

func (s *Service) GetAllLogs(ctx context.Context, page, limit int, search, method, user string, statusCode int) (*dto.PaginatedLogsResponse, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	query := s.db.WithContext(ctx).Model(&middlewares.RequestLog{})
	if search != "" {
		pattern := "%" + search + "%"
		query = query.Where("path ILIKE ? OR \"user\" ILIKE ? OR ip ILIKE ? OR browser ILIKE ? OR os ILIKE ?", pattern, pattern, pattern, pattern, pattern)
	}
	if method != "" {
		query = query.Where("method = ?", method)
	}
	if user != "" {
		query = query.Where("\"user\" = ?", user)
	}
	if statusCode > 0 {
		query = query.Where("status_code = ?", statusCode)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("failed to count logs: %w", err)
	}

	var logs []middlewares.RequestLog
	if err := query.Offset(offset).Limit(limit).Order("created_at desc").Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch logs: %w", err)
	}

	logResponses := make([]dto.LogResponse, len(logs))
	for i, l := range logs {
		logResponses[i] = toLogResponse(&l)
	}

	return &dto.PaginatedLogsResponse{
		Logs:  logResponses,
		Total: total,
		Page:  page,
		Limit: limit,
	}, nil
}
