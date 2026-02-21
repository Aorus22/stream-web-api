package database

import (
	"log"
	"os"
	"path/filepath"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"torrent-stream/internal/model/custom_provider"

	_ "modernc.org/sqlite"
)

// NewDB creates a new database connection
func NewDB(dataDir string) (*gorm.DB, error) {
	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(dataDir, "torrent_stream.db")

	dsn := dbPath + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous=NORMAL"
	db, err := gorm.Open(sqlite.Dialector{
		DriverName: "sqlite",
		DSN:        dsn,
	}, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}

	// Auto migrate tables
	if err := db.AutoMigrate(&custom_provider.CustomProvider{}); err != nil {
		return nil, err
	}

	log.Printf("Database initialized at: %s", dbPath)

	return db, nil
}
