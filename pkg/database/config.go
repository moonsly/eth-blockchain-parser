package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

// Config holds database configuration
type Config struct {
	DatabasePath    string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	PragmaSettings  map[string]string
}

// DefaultConfig returns a production-ready configuration
func DefaultConfig(dbPath string) *Config {
	return &Config{
		DatabasePath:    dbPath,
		MaxOpenConns:    25,
		MaxIdleConns:    5,
		ConnMaxLifetime: time.Hour,
		ConnMaxIdleTime: time.Minute * 5,
		PragmaSettings: map[string]string{
			"journal_mode":       "WAL",         // Write-Ahead Logging for better concurrency
			"synchronous":        "NORMAL",      // Balance between safety and performance
			"cache_size":         "-64000",      // 64MB cache
			"foreign_keys":       "ON",          // Enable foreign key constraints
			"temp_store":         "MEMORY",      // Store temp tables in memory
			"mmap_size":          "268435456",   // 256MB memory-mapped I/O
			"page_size":          "4096",        // 4KB page size
			"auto_vacuum":        "INCREMENTAL", // Incremental auto-vacuum
			"wal_autocheckpoint": "1000",        // Checkpoint after 1000 WAL frames
		},
	}
}

// DatabaseManager handles SQLite connection with auto-reconnection
type DatabaseManager struct {
	db     *sqlx.DB
	config *Config
	logger *log.Logger
}

// NewDatabaseManager creates a new database manager with auto-reconnection
func NewDatabaseManager(config *Config, logger *log.Logger) (*DatabaseManager, error) {
	if logger == nil {
		logger = log.Default()
	}

	dm := &DatabaseManager{
		config: config,
		logger: logger,
	}

	if err := dm.connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return dm, nil
}

// connect establishes a connection to SQLite database
func (dm *DatabaseManager) connect() error {
	// Build connection string with pragmas
	connStr := dm.config.DatabasePath + "?"
	for key, value := range dm.config.PragmaSettings {
		connStr += fmt.Sprintf("_pragma=%s=%s&", key, value)
	}
	// Remove trailing &
	if len(connStr) > 0 && connStr[len(connStr)-1] == '&' {
		connStr = connStr[:len(connStr)-1]
	}

	db, err := sqlx.Connect("sqlite3", connStr)
	if err != nil {
		return fmt.Errorf("failed to connect to SQLite database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(dm.config.MaxOpenConns)
	db.SetMaxIdleConns(dm.config.MaxIdleConns)
	db.SetConnMaxLifetime(dm.config.ConnMaxLifetime)
	db.SetConnMaxIdleTime(dm.config.ConnMaxIdleTime)

	dm.db = db
	dm.logger.Printf("Connected to SQLite database: %s", dm.config.DatabasePath)

	return nil
}

// DB returns the database connection with health check
func (dm *DatabaseManager) DB() (*sqlx.DB, error) {
	if err := dm.Ping(); err != nil {
		dm.logger.Printf("Database connection lost, attempting to reconnect: %v", err)
		if reconnectErr := dm.connect(); reconnectErr != nil {
			return nil, fmt.Errorf("failed to reconnect to database: %w", reconnectErr)
		}
		dm.logger.Println("Successfully reconnected to database")
	}
	return dm.db, nil
}

// Ping checks database connectivity
func (dm *DatabaseManager) Ping() error {
	if dm.db == nil {
		return fmt.Errorf("database connection is nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return dm.db.PingContext(ctx)
}

// Close closes the database connection
func (dm *DatabaseManager) Close() error {
	if dm.db != nil {
		dm.logger.Println("Closing database connection")
		return dm.db.Close()
	}
	return nil
}

// RunInTransaction executes a function within a database transaction
func (dm *DatabaseManager) RunInTransaction(fn func(*sqlx.Tx) error) error {
	db, err := dm.DB()
	if err != nil {
		return err
	}

	tx, err := db.Beginx()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			dm.logger.Printf("Failed to rollback transaction: %v", rollbackErr)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetStats returns database statistics
func (dm *DatabaseManager) GetStats() sql.DBStats {
	if dm.db == nil {
		return sql.DBStats{}
	}
	return dm.db.Stats()
}

// Vacuum performs database maintenance
func (dm *DatabaseManager) Vacuum() error {
	db, err := dm.DB()
	if err != nil {
		return err
	}

	dm.logger.Println("Starting database vacuum operation")
	_, err = db.Exec("VACUUM")
	if err != nil {
		return fmt.Errorf("vacuum operation failed: %w", err)
	}
	dm.logger.Println("Database vacuum completed successfully")

	return nil
}
