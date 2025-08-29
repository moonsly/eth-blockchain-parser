package types

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

// ParsedBlock represents a parsed Ethereum block with additional metadata
type ParsedBlock struct {
	Number       uint64               `json:"number"`
	Hash         string               `json:"hash"`
	ParentHash   string               `json:"parent_hash"`
	Timestamp    time.Time            `json:"timestamp"`
	Miner        string               `json:"miner"`
	GasLimit     uint64               `json:"gas_limit"`
	GasUsed      uint64               `json:"gas_used"`
	BaseFeePerGas *big.Int            `json:"base_fee_per_gas,omitempty"`
	Size         uint64               `json:"size"`
	TxCount      int                  `json:"transaction_count"`
	Transactions []*ParsedTransaction `json:"transactions"`
	UncleCount   int                  `json:"uncle_count"`
}

// ParsedTransaction represents a parsed Ethereum transaction
type ParsedTransaction struct {
	Hash             string             `json:"hash"`
	BlockNumber      uint64             `json:"block_number"`
	BlockHash        string             `json:"block_hash"`
	TransactionIndex uint               `json:"transaction_index"`
	From             string             `json:"from"`
	To               *string            `json:"to"` // nil for contract creation
	Value            *big.Int           `json:"value"`
	Gas              uint64             `json:"gas"`
	GasPrice         *big.Int           `json:"gas_price"`
	GasUsed          uint64             `json:"gas_used"`
	Status           uint64             `json:"status"` // 1 = success, 0 = failure
	InputData        string             `json:"input_data"`
	Nonce            uint64             `json:"nonce"`
	Type             uint8              `json:"type"` // Transaction type (0, 1, 2)
	Logs             []*ParsedLog       `json:"logs,omitempty"`
	ContractAddress  *string            `json:"contract_address,omitempty"`
	
	// EIP-1559 fields
	MaxFeePerGas         *big.Int `json:"max_fee_per_gas,omitempty"`
	MaxPriorityFeePerGas *big.Int `json:"max_priority_fee_per_gas,omitempty"`
}

// ParsedLog represents a parsed Ethereum event log
type ParsedLog struct {
	Address          string   `json:"address"`
	Topics           []string `json:"topics"`
	Data             string   `json:"data"`
	BlockNumber      uint64   `json:"block_number"`
	BlockHash        string   `json:"block_hash"`
	TxHash           string   `json:"transaction_hash"`
	TxIndex          uint     `json:"transaction_index"`
	LogIndex         uint     `json:"log_index"`
	Removed          bool     `json:"removed"`
	DecodedEventName string   `json:"decoded_event_name,omitempty"`
	DecodedData      interface{} `json:"decoded_data,omitempty"`
}

// BlockRange represents a range of blocks to parse
type BlockRange struct {
	Start uint64 `json:"start"`
	End   uint64 `json:"end"`
}

// ParseResult holds the result of parsing operations
type ParseResult struct {
	Block       *ParsedBlock `json:"block,omitempty"`
	Error       error        `json:"error,omitempty"`
	ProcessTime time.Duration `json:"process_time"`
}

// ParsingStats holds statistics about the parsing process
type ParsingStats struct {
	BlocksParsed      uint64        `json:"blocks_parsed"`
	TransactionsParsed uint64        `json:"transactions_parsed"`
	LogsParsed        uint64        `json:"logs_parsed"`
	ErrorsEncountered uint64        `json:"errors_encountered"`
	StartTime         time.Time     `json:"start_time"`
	EndTime           time.Time     `json:"end_time"`
	TotalDuration     time.Duration `json:"total_duration"`
}

// ContractInfo represents smart contract information
type ContractInfo struct {
	Address     string                 `json:"address"`
	Name        string                 `json:"name,omitempty"`
	ABI         interface{}            `json:"abi,omitempty"`
	EventTypes  map[string]interface{} `json:"event_types,omitempty"`
	CreatedAt   uint64                 `json:"created_at,omitempty"`
	Creator     string                 `json:"creator,omitempty"`
}

// Convert go-ethereum types to our parsed types
func NewParsedBlockFromGethBlock(gethBlock *types.Block) *ParsedBlock {
	return &ParsedBlock{
		Number:     gethBlock.NumberU64(),
		Hash:       gethBlock.Hash().Hex(),
		ParentHash: gethBlock.ParentHash().Hex(),
		Timestamp:  time.Unix(int64(gethBlock.Time()), 0),
		Miner:      gethBlock.Coinbase().Hex(),
		GasLimit:   gethBlock.GasLimit(),
		GasUsed:    gethBlock.GasUsed(),
		BaseFeePerGas: gethBlock.BaseFee(),
		Size:       gethBlock.Size(),
		TxCount:    len(gethBlock.Transactions()),
		UncleCount: len(gethBlock.Uncles()),
	}
}

func NewParsedTransactionFromGethTx(gethTx *types.Transaction, blockNumber uint64, blockHash string, txIndex uint) *ParsedTransaction {
	var to *string
	if gethTx.To() != nil {
		toAddr := gethTx.To().Hex()
		to = &toAddr
	}
	
	// Get the from address (requires signature recovery) - handle errors gracefully
	from := ""
	if chainId := gethTx.ChainId(); chainId != nil {
		if msg, err := types.NewEIP155Signer(chainId).Sender(gethTx); err == nil {
			from = msg.Hex()
		} else {
			// Try with other signers for older transactions
msg, err = types.HomesteadSigner{}.Sender(gethTx)
			if err == nil {
					from = msg.Hex()
			}
		}
	}

	// Safely get transaction fields with error handling
	var value *big.Int
	var gasPrice *big.Int
	var inputData string
	var txType uint8

	// Handle transaction value safely
	if gethTx.Value() != nil {
		value = gethTx.Value()
	} else {
		value = big.NewInt(0)
	}

	// Handle gas price safely
	if gethTx.GasPrice() != nil {
		gasPrice = gethTx.GasPrice()
	} else {
		gasPrice = big.NewInt(0)
	}

	// Handle input data safely
	if data := gethTx.Data(); data != nil {
		inputData = common.Bytes2Hex(data)
	}

	// Handle transaction type safely - default to 0 for unknown types
	txType = gethTx.Type()

	return &ParsedTransaction{
		Hash:             gethTx.Hash().Hex(),
		BlockNumber:      blockNumber,
		BlockHash:        blockHash,
		TransactionIndex: txIndex,
		From:             from,
		To:               to,
		Value:            value,
		Gas:              gethTx.Gas(),
		GasPrice:         gasPrice,
		InputData:        inputData,
		Nonce:            gethTx.Nonce(),
		Type:             txType,
	}
}

func NewParsedLogFromGethLog(gethLog *types.Log) *ParsedLog {
	topics := make([]string, len(gethLog.Topics))
	for i, topic := range gethLog.Topics {
		topics[i] = topic.Hex()
	}

	return &ParsedLog{
		Address:     gethLog.Address.Hex(),
		Topics:      topics,
		Data:        common.Bytes2Hex(gethLog.Data),
		BlockNumber: gethLog.BlockNumber,
		BlockHash:   gethLog.BlockHash.Hex(),
		TxHash:      gethLog.TxHash.Hex(),
		TxIndex:     gethLog.TxIndex,
		LogIndex:    gethLog.Index,
		Removed:     gethLog.Removed,
	}
}
