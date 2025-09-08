package database

import (
	"fmt"
	"log"

	"github.com/jmoiron/sqlx"
)

// Schema contains all database schema definitions
type Schema struct {
	logger *log.Logger
}

// NewSchema creates a new schema manager
func NewSchema(logger *log.Logger) *Schema {
	if logger == nil {
		logger = log.Default()
	}
	return &Schema{logger: logger}
}

// CreateAllTables creates all required tables
func (s *Schema) CreateAllTables(db *sqlx.DB) error {
	tables := []struct {
		name   string
		schema string
	}{
		{"transactions", s.transactionsTableSchema()},
		{"whale_addresses", s.whaleAddressesTableSchema()},
	}

	for _, table := range tables {
		s.logger.Printf("Creating table: %s", table.name)
		if _, err := db.Exec(table.schema); err != nil {
			return fmt.Errorf("failed to create table %s: %w", table.name, err)
		}
		s.logger.Printf("Successfully created table: %s", table.name)
	}

	// Create indexes after tables
	if err := s.createIndexes(db); err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}

	s.logger.Println("Database schema created successfully")
	return nil
}

// transactionsTableSchema returns the SQL for creating the transactions table
func (s *Schema) transactionsTableSchema() string {
	return `
	CREATE TABLE IF NOT EXISTS transactions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		tx_hash TEXT NOT NULL UNIQUE,
		block_number INTEGER NOT NULL,
		block_hash TEXT NOT NULL DEFAULT '',
		transaction_index INTEGER NOT NULL,
		from_address TEXT NOT NULL,
		to_address TEXT,
		whale_address_id INTEGER NOT NULL,
		transfer_type TEXT NOT NULL DEFAULT '',
		value DECIMAL(10,5) NOT NULL DEFAULT '0',
		gas INTEGER NOT NULL,
		gas_price TEXT NOT NULL DEFAULT '0',
		gas_used INTEGER,
		status INTEGER,
		nonce INTEGER NOT NULL,
		input_data TEXT,
		tx_type INTEGER NOT NULL DEFAULT 0,
		max_fee_per_gas TEXT,
		max_priority_fee TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (whale_address_id) REFERENCES whale_addresses(id) ON DELETE CASCADE
	);`
}

// addressesTableSchema returns the SQL for creating the addresses table
func (s *Schema) whaleAddressesTableSchema() string {
	return `
	CREATE TABLE IF NOT EXISTS whale_addresses (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		address TEXT NOT NULL UNIQUE,
		label TEXT,
		is_watched BOOLEAN NOT NULL DEFAULT TRUE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
}

// createIndexes creates all necessary indexes for performance
func (s *Schema) createIndexes(db *sqlx.DB) error {
	indexes := []struct {
		name string
		sql  string
	}{
		// Transaction indexes
		{"idx_transactions_from", "CREATE INDEX IF NOT EXISTS idx_transactions_from ON transactions(from_address);"},
		{"idx_transactions_to", "CREATE INDEX IF NOT EXISTS idx_transactions_to ON transactions(to_address);"},
		{"idx_transactions_value", "CREATE INDEX IF NOT EXISTS idx_transactions_value ON transactions(value);"},
		{"idx_transactions_tr_type", "CREATE INDEX IF NOT EXISTS idx_transactions_tr_type ON transactions(transfer_type);"},

		// Address indexes
		{"idx_addresses_address", "CREATE INDEX IF NOT EXISTS idx_addresses_address ON whale_addresses(address);"},
	}

	for _, idx := range indexes {
		s.logger.Printf("Creating index: %s", idx.name)
		if _, err := db.Exec(idx.sql); err != nil {
			return fmt.Errorf("failed to create index %s: %w", idx.name, err)
		}
	}

	s.logger.Println("All indexes created successfully")
	return nil
}

// DropAllTables drops all tables (useful for testing)
func (s *Schema) DropAllTables(db *sqlx.DB) error {
	tables := []string{
		"transactions",
		"whale_addresses",
	}

	for _, table := range tables {
		s.logger.Printf("Dropping table: %s", table)
		if _, err := db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", table)); err != nil {
			return fmt.Errorf("failed to drop table %s: %w", table, err)
		}
	}

	s.logger.Println("All tables dropped successfully")
	return nil
}

// GetTableInfo returns information about all tables
func (s *Schema) GetTableInfo(db *sqlx.DB) (map[string]interface{}, error) {
	info := make(map[string]interface{})

	// Get table list
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("failed to get table list: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			continue
		}
		tables = append(tables, tableName)
	}
	info["tables"] = tables

	// Get database size
	var pageCount, pageSize int64
	err = db.QueryRow("PRAGMA page_count").Scan(&pageCount)
	if err == nil {
		err = db.QueryRow("PRAGMA page_size").Scan(&pageSize)
		if err == nil {
			info["database_size_bytes"] = pageCount * pageSize
		}
	}

	// Get table counts
	tableCounts := make(map[string]int64)
	for _, table := range tables {
		var count int64
		err := db.Get(&count, fmt.Sprintf("SELECT COUNT(*) FROM %s", table))
		if err == nil {
			tableCounts[table] = count
		}
	}
	info["table_counts"] = tableCounts

	return info, nil
}
