package database

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"net-http/internal/config"
	"net-http/internal/utils"
)

// ConnectDB initializes the GORM PostgreSQL connection and returns it.
//
// It deliberately does not publish the connection through a package-level
// `var DB`. That global was the single hardest dependency to shake: any package
// could (and did) reach into database.DB, which hid the real dependency graph and
// made it impossible to give one caller a different connection. Returning the
// handle lets the composition root inject it only where it is needed.
func ConnectDB() *gorm.DB {
	logger := utils.NewLogger("DATABASE")

	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		config.AppConfig.DB_HOST,
		config.AppConfig.DB_USER,
		config.AppConfig.DB_PASSWORD,
		config.AppConfig.DB_NAME,
		config.AppConfig.DB_PORT,
		config.AppConfig.DB_SSLMODE,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		logger.Fatalf("Failed to connect to database: %v", err)
	}

	logger.Println("Successfully connected to PostgreSQL database!")

	return db
}

// AutoMigrate runs GORM auto migration for all registered entities.
//
// The connection is passed in rather than read from the old global so the
// migration target is explicit and a test can migrate a throwaway database.
func AutoMigrate(db *gorm.DB, models ...any) {
	logger := utils.NewLogger("DATABASE")
	if db == nil {
		logger.Fatalf("Database connection is not initialized")
	}

	err := db.AutoMigrate(models...)
	if err != nil {
		logger.Fatalf("Failed to run database migration: %v", err)
	}

	logger.Println("Database migration completed successfully!")
}
