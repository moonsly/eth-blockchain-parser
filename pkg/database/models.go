package database

import (
	"database/sql/driver"
	"eth-blockchain-parser/internal/types"
	"fmt"
	"strconv"
	"time"
)

// Transaction represents a blockchain transaction
// Matches the actual database schema with all required fields
type Transaction struct {
	ID               int64     `json:"id" db:"id"`
	TxHash           string    `json:"tx_hash" db:"tx_hash"`
	BlockNumber      int64     `json:"block_number" db:"block_number"`
	BlockHash        string    `json:"block_hash" db:"block_hash"`
	TransactionIndex int64     `json:"transaction_index" db:"transaction_index"`
	FromAddress      string    `json:"from_address" db:"from_address"`
	ToAddress        *string   `json:"to_address" db:"to_address"`             // Nullable for contract creation
	WhaleAddressID   int64     `json:"whale_address_id" db:"whale_address_id"` // Foreign key - required field
	TransferType     string    `json:"transfer_type" db:"transfer_type"`       // Required field with default ''
	Value            string    `json:"value" db:"value"`                       // Store as string, DB has DECIMAL(10,5) with default '0'
	Gas              int64     `json:"gas" db:"gas"`
	GasPrice         string    `json:"gas_price" db:"gas_price"` // Default '0'
	GasUsed          *int64    `json:"gas_used" db:"gas_used"`   // Nullable if not yet mined
	Status           *int      `json:"status" db:"status"`       // Nullable, 0=failed, 1=success
	Nonce            int64     `json:"nonce" db:"nonce"`
	InputData        *string   `json:"input_data" db:"input_data"`             // BLOB field
	TxType           int       `json:"tx_type" db:"tx_type"`                   // Default 0
	MaxFeePerGas     *string   `json:"max_fee_per_gas" db:"max_fee_per_gas"`   // EIP-1559, nullable
	MaxPriorityFee   *string   `json:"max_priority_fee" db:"max_priority_fee"` // EIP-1559, nullable
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
}

// SetDefaults sets default values for required fields
func (t *Transaction) SetDefaults() {
	if t.BlockHash == "" {
		t.BlockHash = ""
	}
	if t.TransferType == "" {
		t.TransferType = ""
	}
	if t.Value == "" {
		t.Value = "0"
	}
	if t.GasPrice == "" {
		t.GasPrice = "0"
	}
	if t.WhaleAddressID == 0 {
		// Set to 1 as default whale address ID
		// This should be handled by the mapper function
		t.WhaleAddressID = 1
	}
}

// MapParsedTxToDatabaseTx converts a types.ParsedTransaction to database.Transaction
// The whaleAddressID parameter should be obtained from the whale_addresses table
func MapParsedTxToDatabaseTx(parsedTx *types.ParsedTransaction, params ...string) (*Transaction, error) {
	var value string
	if parsedTx.Value != nil {
		value = parsedTx.Value.String()
	} else {
		value = "0"
	}

	var gasPrice string
	if parsedTx.GasPrice != nil {
		gasPrice = parsedTx.GasPrice.String()
	} else {
		gasPrice = "0"
	}

	// Handle optional EIP-1559 fields
	var maxFeePerGas *string
	if parsedTx.MaxFeePerGas != nil {
		maxFeeStr := parsedTx.MaxFeePerGas.String()
		maxFeePerGas = &maxFeeStr
	}

	var maxPriorityFee *string
	if parsedTx.MaxPriorityFeePerGas != nil {
		maxPriorityFeeStr := parsedTx.MaxPriorityFeePerGas.String()
		maxPriorityFee = &maxPriorityFeeStr
	}

	// Handle nullable fields
	var gasUsed *int64
	if parsedTx.GasUsed > 0 {
		gasUsedVal := int64(parsedTx.GasUsed)
		gasUsed = &gasUsedVal
	}

	var status *int
	if parsedTx.Status == 1 || parsedTx.Status == 0 {
		statusVal := int(parsedTx.Status)
		status = &statusVal
	}

	// Create the database transaction
	tx := &Transaction{
		TxHash:           parsedTx.Hash,
		BlockNumber:      int64(parsedTx.BlockNumber),
		BlockHash:        parsedTx.BlockHash,
		TransactionIndex: int64(parsedTx.TransactionIndex),
		FromAddress:      parsedTx.From,
		ToAddress:        parsedTx.To, // This is already *string
		WhaleAddressID:   0,
		TransferType:     "", // Default empty string
		Value:            value,
		Gas:              int64(parsedTx.Gas),
		GasPrice:         gasPrice,
		GasUsed:          gasUsed,
		Status:           status,
		Nonce:            int64(parsedTx.Nonce),
		InputData:        &parsedTx.InputData,
		TxType:           int(parsedTx.Type),
		MaxFeePerGas:     maxFeePerGas,
		MaxPriorityFee:   maxPriorityFee,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	// value 1.12345, from/to, whale_id, from/to_addr
	for i, prm := range params {
		switch i {
		case 0:
			tx.Value = prm
		case 1:
			tx.TransferType = prm
		case 2:
			whaleAddressID, err := strconv.Atoi(prm)
			if err != nil {
				return tx, fmt.Errorf("Error converting %s to int", prm)
			}
			tx.WhaleAddressID = int64(whaleAddressID)
		case 3:
		}
	}

	// Set defaults for required fields
	tx.SetDefaults()
	fmt.Println("MAPPED", tx.Value, tx.TransferType, tx.WhaleAddressID)

	return tx, nil
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
