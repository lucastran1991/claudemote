package database

import (
	"fmt"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Connect opens a SQLite database at the given file path with WAL mode enabled.
// WAL allows concurrent reads alongside writes — important for SSE log streaming.
func Connect(dbPath string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("open database %s: %w", dbPath, err)
	}

	// Enable WAL mode for better concurrency under the worker pool.
	if err := db.Exec("PRAGMA journal_mode=WAL").Error; err != nil {
		return nil, fmt.Errorf("enable WAL mode: %w", err)
	}

	// Enable foreign key enforcement (SQLite disables it by default).
	if err := db.Exec("PRAGMA foreign_keys=ON").Error; err != nil {
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	return db, nil
}
