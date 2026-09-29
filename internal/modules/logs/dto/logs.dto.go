package dto

import "time"

type LogResponse struct {
	ID         uint      `json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	Method     string    `json:"method"`
	Path       string    `json:"path"`
	StatusCode int       `json:"status_code"`
	DurationMs int64     `json:"duration_ms"`
	IP         string    `json:"ip"`
	OS         string    `json:"os"`
	Browser    string    `json:"browser"`
	UserAgent  string    `json:"user_agent"`
	User       string    `json:"user"`
}

type PaginatedLogsResponse struct {
	Logs  []LogResponse `json:"logs"`
	Total int64         `json:"total"`
	Page  int           `json:"page"`
	Limit int           `json:"limit"`
}
