// Package database provides SQLite connection and migration runner.
//
// SAFETY CONSTRAINTS:
//  1. Single-writer assumption. Run migrations from ONE process at a time.
//  2. Migration files are immutable once applied — never rename/edit an applied migration.
//  3. Each migration file must contain exactly one logical DDL statement group.
package database

import (
	"embed"
	"fmt"
	"sort"
	"strings"

	"gorm.io/gorm"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// RunMigrations applies any pending up-migrations in alphabetical order.
// Tracks applied versions in schema_migrations table.
// Safe to call on every boot — no-op if already up to date.
func RunMigrations(db *gorm.DB) error {
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version    TEXT     PRIMARY KEY,
		applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`).Error; err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	var upFiles []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".up.sql") {
			upFiles = append(upFiles, e.Name())
		}
	}
	sort.Strings(upFiles)

	for _, name := range upFiles {
		version := strings.TrimSuffix(name, ".up.sql")

		var count int64
		if err := db.Raw("SELECT COUNT(*) FROM schema_migrations WHERE version = ?", version).
			Scan(&count).Error; err != nil {
			return fmt.Errorf("check version %s: %w", version, err)
		}
		if count > 0 {
			continue // already applied
		}

		sql, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}

		err = db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec(string(sql)).Error; err != nil {
				return fmt.Errorf("apply %s: %w", name, err)
			}
			if err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", version).Error; err != nil {
				return fmt.Errorf("record %s: %w", name, err)
			}
			return nil
		})
		if err != nil {
			return err
		}
		fmt.Printf("migration applied: %s\n", version)
	}
	return nil
}
