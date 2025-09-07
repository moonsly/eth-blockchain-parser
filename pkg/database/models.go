package database

import (
	"database/sql/driver"
	"fmt"
	"time"
)

// Transaction represents a blockchain transaction
type Transaction struct {
	ID               int64     `json:"id" db:"id"`
	TxHash           string    `json:"tx_hash" db:"tx_hash"`
	BlockNumber      int64     `json:"block_number" db:"block_number"`
	BlockHash        string    `json:"block_hash" db:"block_hash"`
	TransactionIndex int       `json:"transaction_index" db:"transaction_index"`
	FromAddress      string    `json:"from_address" db:"from_address"`
	ToAddress        *string   `json:"to_address" db:"to_address"` // Nullable for contract creation
	Value            string    `json:"value" db:"value"`           // Store as string to avoid precision loss
	Gas              int64     `json:"gas" db:"gas"`
	GasPrice         string    `json:"gas_price" db:"gas_price"`
	GasUsed          *int64    `json:"gas_used" db:"gas_used"` // Nullable if not yet mined
	Status           *int      `json:"status" db:"status"`     // Nullable, 0=failed, 1=success
	Nonce            int64     `json:"nonce" db:"nonce"`
	InputData        []byte    `json:"input_data" db:"input_data"`
	TxType           int       `json:"tx_type" db:"tx_type"`                   // Transaction type (0=legacy, 1=EIP-2930, 2=EIP-1559)
	MaxFeePerGas     *string   `json:"max_fee_per_gas" db:"max_fee_per_gas"`   // EIP-1559
	MaxPriorityFee   *string   `json:"max_priority_fee" db:"max_priority_fee"` // EIP-1559
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
}

// Address represents an Ethereum address with metadata
type WhaleAddress struct {
	ID        int64     `json:"id" db:"id"`
	Address   string    `json:"address" db:"address"`
	Label     *string   `json:"label" db:"label"` // Optional human-readable label
	IsWatched bool      `json:"is_watched" db:"is_watched"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// Custom scanner for handling nullable string slices (topics)
type NullableStringSlice []string

func (ns *NullableStringSlice) Scan(value interface{}) error {
	if value == nil {
		*ns = nil
		return nil
	}

	switch v := value.(type) {
	case string:
		if v == "" {
			*ns = nil
		} else {
			*ns = []string{v}
		}
	case []byte:
		if len(v) == 0 {
			*ns = nil
		} else {
			*ns = []string{string(v)}
		}
	default:
		return fmt.Errorf("cannot scan %T into NullableStringSlice", value)
	}

	return nil
}

func (ns NullableStringSlice) Value() (driver.Value, error) {
	if ns == nil || len(ns) == 0 {
		return nil, nil
	}
	return ns[0], nil
}

// TableNames contains all table names for easy reference
var TableNames = struct {
	Transactions   string
	WhaleAddresses string
}{
	Transactions:   "transactions",
	WhaleAddresses: "whale_addresses",
}
