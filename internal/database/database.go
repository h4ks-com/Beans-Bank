package database

import (
	"fmt"
	"log"

	"github.com/h4ks-com/beapin/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Connect(databaseURL string) (*gorm.DB, error) {
	var db *gorm.DB
	var err error

	config := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	}

	if databaseURL == "" || databaseURL == ":memory:" {
		db, err = gorm.Open(sqlite.Open(":memory:"), config)
	} else if len(databaseURL) > 10 && databaseURL[:6] == "sqlite" {
		// Strip "sqlite:" prefix for SQLite driver
		dbPath := databaseURL[7:]
		// Add query parameters to ensure write access
		dbPath = dbPath + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)"
		db, err = gorm.Open(sqlite.Open(dbPath), config)
	} else {
		db, err = gorm.Open(postgres.Open(databaseURL), config)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return db, nil
}

func Migrate(db *gorm.DB) error {
	log.Println("Running database migrations...")

	err := db.AutoMigrate(
		&models.User{},
		&models.Transaction{},
		&models.APIToken{},
	)

	if err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	log.Println("Database migrations completed successfully")
	return nil
}
