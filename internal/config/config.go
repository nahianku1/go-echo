package config

import (
	"github.com/joho/godotenv"
	"os"
)

var err = godotenv.Load()

type Config struct {
	PORT       string
	HOST       string
	JWT_SECRET string
	DB_URI     string

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

func getEnv(key string) string {
	return os.Getenv(key)

}

var AppConfig Config = Config{
	PORT:                getEnv("PORT"),
	HOST:                getEnv("HOST"),
	JWT_SECRET:          getEnv("JWT_SECRET"),
	DB_URI:              getEnv("DB_URI"),
	GOOGLE_CLIENT_ID:    getEnv("GOOGLE_CLIENT_ID"),
	GOOGLE_REDIRECT_URL: getEnv("GOOGLE_REDIRECT_URL"),

	GOOGLE_CLIENT_SECRET: getEnv("GOOGLE_CLIENT_SECRET"),
	SMTP_HOST:            getEnv("SMTP_HOST"),
	SMTP_PORT:            getEnv("SMTP_PORT"),
	SMTP_USER:            getEnv("SMTP_USER"),
	SMTP_PASSWORD:        getEnv("SMTP_PASSWORD"),
	SMTP_FROM:            getEnv("SMTP_FROM"),
	FRONTEND_URL:         getEnv("FRONTEND_URL"),
}
