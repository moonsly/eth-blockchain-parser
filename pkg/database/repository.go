package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

// Repository provides database operations with auto-reconnection
type Repository struct {
	dm     *DatabaseManager
	logger *log.Logger
}

// NewRepository creates a new repository instance
func NewRepository(dm *DatabaseManager, logger *log.Logger) *Repository {
	if logger == nil {
		logger = log.Default()
	}
	return &Repository{
		dm:     dm,
		logger: logger,
	}
}

// TransactionRepository handles transaction-related database operations
type TransactionRepository struct {
	*Repository
}

// NewTransactionRepository creates a new transaction repository
func NewTransactionRepository(dm *DatabaseManager, logger *log.Logger) *TransactionRepository {
	return &TransactionRepository{
		Repository: NewRepository(dm, logger),
	}
}

// Insert inserts a new transaction
func (tr *TransactionRepository) Insert(ctx context.Context, tx *Transaction) error {
	db, err := tr.dm.DB()
	if err != nil {
		return fmt.Errorf("failed to get database connection: %w", err)
	}

	tx.CreatedAt = time.Now()
	tx.UpdatedAt = time.Now()

	query := `
		INSERT INTO transactions (
			tx_hash, block_number, transaction_index, from_address, to_address,
			value, gas, gas_price, gas_used, status, nonce, input_data, tx_type,
			max_fee_per_gas, max_priority_fee, created_at, updated_at
		) VALUES (
			:tx_hash, :block_number, :transaction_index, :from_address, :to_address,
			:value, :gas, :gas_price, :gas_used, :status, :nonce, :input_data, :tx_type,
			:max_fee_per_gas, :max_priority_fee, :created_at, :updated_at
		)`

	result, err := db.NamedExecContext(ctx, query, tx)
	if err != nil {
		return fmt.Errorf("failed to insert transaction: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert ID: %w", err)
	}
	tx.ID = id

	tr.logger.Printf("Inserted transaction %s", tx.TxHash)
	return nil
}

// GetByHash retrieves a transaction by its hash
func (tr *TransactionRepository) GetByHash(ctx context.Context, txHash string) (*Transaction, error) {
	db, err := tr.dm.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database connection: %w", err)
	}

	var tx Transaction
	query := "SELECT * FROM transactions WHERE tx_hash = ? LIMIT 1"

	err = db.GetContext(ctx, &tx, query, txHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get transaction by hash %s: %w", txHash, err)
	}

	return &tx, nil
}

// GetByAddress retrieves transactions for a specific address (from or to)
func (tr *TransactionRepository) GetByAddress(ctx context.Context, address string, limit int, offset int) ([]*Transaction, error) {
	db, err := tr.dm.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database connection: %w", err)
	}

	query := `
		SELECT * FROM transactions 
		WHERE from_address = ? OR to_address = ? 
		ORDER BY block_number DESC, transaction_index DESC 
		LIMIT ? OFFSET ?`

	var transactions []*Transaction
	err = db.SelectContext(ctx, &transactions, query, address, address, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get transactions for address %s: %w", address, err)
	}

	return transactions, nil
}

// GetByBlockNumber retrieves all transactions in a block
func (tr *TransactionRepository) GetByBlockNumber(ctx context.Context, blockNumber int64) ([]*Transaction, error) {
	db, err := tr.dm.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database connection: %w", err)
	}

	query := `
		SELECT * FROM transactions 
		WHERE block_number = ? 
		ORDER BY transaction_index ASC`

	var transactions []*Transaction
	err = db.SelectContext(ctx, &transactions, query, blockNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get transactions for block %d: %w", blockNumber, err)
	}

	return transactions, nil
}

// BatchInsert inserts multiple transactions in a transaction
func (tr *TransactionRepository) BatchInsert(ctx context.Context, transactions []*Transaction) error {
	if len(transactions) == 0 {
		return nil
	}

	return tr.dm.RunInTransaction(func(tx *sqlx.Tx) error {
		query := `
			INSERT OR REPLACE INTO transactions (
				tx_hash, block_number, block_hash, transaction_index, from_address, to_address,
				value, gas, gas_price, gas_used, status, nonce, input_data, tx_type, transfer_type,
				max_fee_per_gas, max_priority_fee, created_at, updated_at, whale_address_id
			) VALUES (
				:tx_hash, :block_number, :block_hash, :transaction_index, :from_address, :to_address,
				:value, :gas, :gas_price, :gas_used, :status, :nonce, :input_data, :tx_type, :transfer_type,
				:max_fee_per_gas, :max_priority_fee, :created_at, :updated_at, :whale_address_id
			)`

		now := time.Now()
		for _, transaction := range transactions {
			if transaction.CreatedAt.IsZero() {
				transaction.CreatedAt = now
			}
			transaction.UpdatedAt = now
		}

		_, err := tx.NamedExecContext(ctx, query, transactions)
		if err != nil {
			return fmt.Errorf("failed to batch insert transactions: %w", err)
		}

		tr.logger.Printf("Batch inserted %d transactions", len(transactions))
		return nil
	})
}

// AddressRepository handles address-related database operations
type AddressRepository struct {
	*Repository
}

// NewAddressRepository creates a new address repository
func NewAddressRepository(dm *DatabaseManager, logger *log.Logger) *AddressRepository {
	return &AddressRepository{
		Repository: NewRepository(dm, logger),
	}
}

// InsertOrUpdate inserts a new address or updates existing one
func (ar *AddressRepository) InsertOrUpdate(ctx context.Context, address *WhaleAddress) error {
	db, err := ar.dm.DB()
	if err != nil {
		return fmt.Errorf("failed to get database connection: %w", err)
	}

	now := time.Now()
	address.UpdatedAt = now

	// Check if address exists
	var existingID sql.NullInt64
	err = db.GetContext(ctx, &existingID, "SELECT id FROM whale_addresses WHERE address = ?", address.Address)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to check existing address: %w", err)
	}

	if existingID.Valid {
		// Update existing address
		address.ID = existingID.Int64
		query := `
			UPDATE whale_addresses SET 
				label = :label,
				address_type = :address_type,
				is_watched = :is_watched,
				updated_at = :updated_at
			WHERE id = :id`

		_, err = db.NamedExecContext(ctx, query, address)
		if err != nil {
			return fmt.Errorf("failed to update address: %w", err)
		}
		ar.logger.Printf("Updated address %s", address.Address)
	} else {
		// Insert new address
		if address.CreatedAt.IsZero() {
			address.CreatedAt = now
		}

		query := `
			INSERT INTO whale_addresses (
				address, label, is_watched, created_at, updated_at
			) VALUES (
				:address, :label, :is_watched, :created_at, :updated_at
			)`

		result, err := db.NamedExecContext(ctx, query, address)
		if err != nil {
			return fmt.Errorf("failed to insert address: %w", err)
		}

		id, err := result.LastInsertId()
		if err != nil {
			return fmt.Errorf("failed to get last insert ID: %w", err)
		}
		address.ID = id
		ar.logger.Printf("Inserted address %s", address.Address)
	}

	return nil
}

// GetWatched retrieves all watched whale_addresses
func (ar *AddressRepository) GetWatched(ctx context.Context) ([]*WhaleAddress, error) {
	db, err := ar.dm.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database connection: %w", err)
	}

	query := "SELECT * FROM whale_addresses WHERE is_watched = TRUE ORDER BY created_at DESC"

	var addresses []*WhaleAddress
	err = db.SelectContext(ctx, &addresses, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get watched addresses: %w", err)
	}

	return addresses, nil
}

// Search searches for addresses by label or address
func (ar *AddressRepository) Search(ctx context.Context, searchTerm string, limit int) ([]*WhaleAddress, error) {
	db, err := ar.dm.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database connection: %w", err)
	}

	searchPattern := "%" + strings.ToLower(searchTerm) + "%"
	query := `
		SELECT * FROM whale_addresses 
		WHERE LOWER(address) LIKE ? 
		   OR LOWER(label) LIKE ?
		ORDER BY created_at DESC 
		LIMIT ?`

	var addresses []*WhaleAddress
	err = db.SelectContext(ctx, &addresses, query, searchPattern, searchPattern, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search addresses: %w", err)
	}

	return addresses, nil
}
