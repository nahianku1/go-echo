package config

import (
	"github.com/joho/godotenv"
	"os"
)

var err = godotenv.Load()


type Config struct {
	PORT                 string
	HOST                 string
	JWT_SECRET           string
	DB_HOST              string
	DB_PORT              string
	DB_USER              string
	DB_PASSWORD          string
	DB_NAME              string
	DB_SSLMODE           string
	GOOGLE_CLIENT_ID     string
	GOOGLE_CLIENT_SECRET string
	GOOGLE_REDIRECT_URL  string
	SMTP_HOST            string
	SMTP_PORT            string
	SMTP_USER            string
	SMTP_PASSWORD        string
	SMTP_FROM            string
	FRONTEND_URL         string
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

var AppConfig Config = Config{
	PORT:                getEnv("PORT", "4000"),
	HOST:                getEnv("HOST", "localhost"),
	JWT_SECRET:          getEnv("JWT_SECRET", "my-secret-key"),
	DB_HOST:             getEnv("DB_HOST", "localhost"),
	DB_PORT:             getEnv("DB_PORT", "5432"),
	DB_USER:             getEnv("DB_USER", "postgres"),
	DB_PASSWORD:         getEnv("DB_PASSWORD", "postgres"),
	DB_NAME:             getEnv("DB_NAME", "net_http_db"),
	DB_SSLMODE:          getEnv("DB_SSLMODE", "disable"),
	GOOGLE_CLIENT_ID:    getEnv("GOOGLE_CLIENT_ID", "691283293069-rlp9tv47etof6ipgoge4qegsudn8nto1.apps.googleusercontent.com"),
	GOOGLE_REDIRECT_URL: getEnv("GOOGLE_REDIRECT_URL", "http://localhost:5173/"),

	GOOGLE_CLIENT_SECRET: getEnv("GOOGLE_CLIENT_SECRET", "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"),
	SMTP_HOST:            getEnv("SMTP_HOST", "live.smtp.mailtrap.io"),
	SMTP_PORT:            getEnv("SMTP_PORT", "587"),
	SMTP_USER:            getEnv("SMTP_USER", "api"),
	SMTP_PASSWORD:        getEnv("SMTP_PASSWORD", ""),
	SMTP_FROM:            getEnv("SMTP_FROM", "hello@demomailtrap.com"),
	FRONTEND_URL:         getEnv("FRONTEND_URL", "http://localhost:5173"),
}
