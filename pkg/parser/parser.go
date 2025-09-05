package parser

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"sync"
	"time"

	"eth-blockchain-parser/pkg/client"
	"eth-blockchain-parser/pkg/types"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
)

// Parser handles blockchain data parsing
type Parser struct {
	client *client.EthClient
	config *types.Config
	stats  *types.ParsingStats
	mu     sync.RWMutex
}

// NewParser creates a new blockchain parser
func NewParser(ethClient *client.EthClient, config *types.Config) *Parser {
	return &Parser{
		client: ethClient,
		config: config,
		stats: &types.ParsingStats{
			StartTime: time.Now(),
		},
	}
}

// ParseBlockRange parses a range of blocks
func (p *Parser) ParseBlockRange(ctx context.Context, startBlock, endBlock uint64) ([]*types.ParsedBlock, error) {
	log.Printf("Parsing blocks from %d to %d", startBlock, endBlock)

	p.mu.Lock()
	p.stats.StartTime = time.Now()
	p.mu.Unlock()

	var allBlocks []*types.ParsedBlock
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Create worker pool
	blockChan := make(chan uint64, p.config.Workers*2)
	resultChan := make(chan *types.ParseResult, p.config.Workers)

	// Start workers
	for i := 0; i < p.config.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// TODO: pass to every worker separate infura API key
			p.worker(ctx, blockChan, resultChan)
		}()
	}

	// Start result collector
	go func() {
		for result := range resultChan {
			if result.Error != nil {
				log.Printf("Error parsing block: %v", result.Error)
				p.mu.Lock()
				p.stats.ErrorsEncountered++
				p.mu.Unlock()
				continue
			}

			mu.Lock()
			allBlocks = append(allBlocks, result.Block)
			mu.Unlock()

			p.mu.Lock()
			p.stats.BlocksParsed++
			if result.Block != nil {
				p.stats.TransactionsParsed += uint64(len(result.Block.Transactions))
				for _, tx := range result.Block.Transactions {
					if tx.Logs != nil {
						p.stats.LogsParsed += uint64(len(tx.Logs))
					}
				}
			}
			p.mu.Unlock()
		}
	}()

	// Send block numbers to workers
	go func() {
		defer close(blockChan)
		for blockNum := startBlock; blockNum <= endBlock; blockNum++ {
			select {
			case blockChan <- blockNum:
			case <-ctx.Done():
				return
			}
		}
	}()

	// Wait for all workers to complete
	wg.Wait()
	close(resultChan)

	p.mu.Lock()
	p.stats.EndTime = time.Now()
	p.stats.TotalDuration = p.stats.EndTime.Sub(p.stats.StartTime)
	p.mu.Unlock()

	log.Printf("Parsing completed. Processed %d blocks, %d transactions, %d logs",
		p.stats.BlocksParsed, p.stats.TransactionsParsed, p.stats.LogsParsed)

	return allBlocks, nil
}

// ParseSingleBlock parses a single block by number
func (p *Parser) ParseSingleBlock(ctx context.Context, blockNumber uint64) (*types.ParsedBlock, error) {
	startTime := time.Now()

	// Get block data
	gethBlock, err := p.client.GetBlockByNumber(ctx, blockNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get block %d: %w", blockNumber, err)
	}

	// Convert to parsed block
	parsedBlock := types.NewParsedBlockFromGethBlock(gethBlock)

	// Parse transactions
	transactions, err := p.parseBlockTransactions(ctx, gethBlock)
	if err != nil {
		return nil, fmt.Errorf("failed to parse transactions for block %d: %w", blockNumber, err)
	}
	parsedBlock.Transactions = transactions

	// Check if we should skip receipts for large blocks
	if p.config.SkipReceiptsOnLargeBlocks && len(transactions) > p.config.MaxTransactionsForReceipts {
		log.Printf("Skipping receipt processing for block %d: %d transactions exceeds limit of %d",
			blockNumber, len(transactions), p.config.MaxTransactionsForReceipts)
		// Set basic transaction info without receipts
		for _, tx := range transactions {
			tx.GasUsed = 0
			tx.Status = 2 // Use 2 to indicate "receipt not fetched"
		}
	}

	log.Printf("Parsed block %d with %d transactions in %v",
		blockNumber, len(transactions), time.Since(startTime))

	return parsedBlock, nil
}

// parseBlockTransactions parses all transactions in a block
func (p *Parser) parseBlockTransactions(ctx context.Context, gethBlock *gethTypes.Block) ([]*types.ParsedTransaction, error) {
	blockTxs := gethBlock.Transactions()
	if len(blockTxs) == 0 {
		return []*types.ParsedTransaction{}, nil
	}

	var parsedTxs []*types.ParsedTransaction
	// Check if we should skip receipts for large blocks
	if p.config.SkipReceiptsOnLargeBlocks && len(blockTxs) > p.config.MaxTransactionsForReceipts {
		log.Printf("Skipping receipts for block with %d transactions (exceeds limit of %d)",
			len(blockTxs), p.config.MaxTransactionsForReceipts)
		// Parse transactions without receipts
		for i, gethTx := range blockTxs {
			parsedTx, err := p.parseTransactionWithoutReceipt(gethTx, gethBlock, uint(i))
			if err != nil {
				log.Printf("Warning: Failed to parse transaction %s: %v", gethTx.Hash().Hex(), err)
				continue
			}
			parsedTxs = append(parsedTxs, parsedTx)
		}
		return parsedTxs, nil
	}

	// Get transaction receipts in batch for smaller blocks
	txHashes := make([]common.Hash, len(blockTxs))
	for i, tx := range blockTxs {
		txHashes[i] = tx.Hash()
	}

	if p.config.IncludeLogs {
		receipts, err := p.client.GetTransactionReceiptsBatch(ctx, txHashes)
		if err != nil {
			return nil, fmt.Errorf("failed to get transaction receipts: %w", err)
		}

		// Parse each transaction with error handling
		var parsedTxs []*types.ParsedTransaction
		for i, gethTx := range blockTxs {
			// Try to parse transaction, skip if it fails
			parsedTx, err := p.parseTransactionSafely(gethTx, gethBlock, uint(i), receipts, i)
			if err != nil {
				log.Printf("Warning: Failed to parse transaction %s in block %d: %v (skipping)",
					gethTx.Hash().Hex(), gethBlock.NumberU64(), err)
				// Create a minimal transaction record for unknown types
				parsedTx = &types.ParsedTransaction{
					Hash:             gethTx.Hash().Hex(),
					BlockNumber:      gethBlock.NumberU64(),
					BlockHash:        gethBlock.Hash().Hex(),
					TransactionIndex: uint(i),
					From:             "unknown",
					Value:            big.NewInt(0),
					Gas:              0,
					GasPrice:         big.NewInt(0),
					Nonce:            0,
					Type:             255, // Use 255 to indicate unknown/unsupported type (e.g., blob txs)
					InputData:        "parse_error",
				}
			}
			parsedTxs = append(parsedTxs, parsedTx)
		}
		return parsedTxs, nil
	}
	return parsedTxs, nil

}

// parseTransactionSafely safely parses a transaction with error handling for unknown types
func (p *Parser) parseTransactionSafely(gethTx *gethTypes.Transaction, gethBlock *gethTypes.Block, txIndex uint, receipts []*gethTypes.Receipt, receiptIndex int) (*types.ParsedTransaction, error) {
	// Try to parse the transaction with error recovery
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic while parsing transaction %s: %v", gethTx.Hash().Hex(), r)
		}
	}()

	// Basic transaction parsing with safe field access
	var to *string
	if gethTx.To() != nil {
		toAddr := gethTx.To().Hex()
		to = &toAddr
	}

	// Safe from address extraction
	from := "unknown"
	txType := gethTx.Type()

	// Try different signer types for different transaction types
	if chainId := gethTx.ChainId(); chainId != nil && chainId.Sign() != 0 {
		// Try EIP-155 signer first
		if msg, err := gethTypes.NewEIP155Signer(chainId).Sender(gethTx); err == nil {
			from = msg.Hex()
		} else {
			// Fallback to other signers
			if msg, err := gethTypes.LatestSignerForChainID(chainId).Sender(gethTx); err == nil {
				from = msg.Hex()
			} else {
				signer := gethTypes.HomesteadSigner{}
				if msg, err := signer.Sender(gethTx); err == nil {
					from = msg.Hex()
				}
			}
		}
	}

	// Safe value access
	value := big.NewInt(0)
	if gethTx.Value() != nil {
		value = gethTx.Value()
	}

	// Safe gas price access
	gasPrice := big.NewInt(0)
	if gethTx.GasPrice() != nil {
		gasPrice = gethTx.GasPrice()
	}

	// Safe input data access
	inputData := ""
	if data := gethTx.Data(); data != nil {
		inputData = common.Bytes2Hex(data)
	}

	parsedTx := &types.ParsedTransaction{
		Hash:             gethTx.Hash().Hex(),
		BlockNumber:      gethBlock.NumberU64(),
		BlockHash:        gethBlock.Hash().Hex(),
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

	// Add receipt data if available
	if receiptIndex < len(receipts) && receipts[receiptIndex] != nil {
		receipt := receipts[receiptIndex]
		parsedTx.GasUsed = receipt.GasUsed
		parsedTx.Status = receipt.Status

		// Add contract address if this is a contract creation
		if receipt.ContractAddress != (common.Address{}) {
			contractAddr := receipt.ContractAddress.Hex()
			parsedTx.ContractAddress = &contractAddr
		}

		// Parse logs if enabled
		if p.config.IncludeLogs && len(receipt.Logs) > 0 {
			parsedTx.Logs = make([]*types.ParsedLog, len(receipt.Logs))
			for j, gethLog := range receipt.Logs {
				parsedTx.Logs[j] = types.NewParsedLogFromGethLog(gethLog)
			}
		}
	}

	// Safely add EIP-1559 fields for type 2 transactions
	// Also handle new transaction types introduced in go-ethereum 1.16+
	if txType == 2 {
		// Use defer/recover to handle any panics from accessing EIP-1559 fields
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Error accessing EIP-1559 fields for tx %s: %v", gethTx.Hash().Hex(), r)
				}
			}()

			if gasFeeCap := gethTx.GasFeeCap(); gasFeeCap != nil {
				parsedTx.MaxFeePerGas = gasFeeCap
			}
			if gasTipCap := gethTx.GasTipCap(); gasTipCap != nil {
				parsedTx.MaxPriorityFeePerGas = gasTipCap
			}
		}()
	}

	return parsedTx, nil
}

// worker processes block numbers from the channel
func (p *Parser) worker(ctx context.Context, blockChan <-chan uint64, resultChan chan<- *types.ParseResult) {
	for {
		select {
		case blockNum, ok := <-blockChan:
			if !ok {
				return
			}

			startTime := time.Now()
			block, err := p.ParseSingleBlock(ctx, blockNum)

			resultChan <- &types.ParseResult{
				Block:       block,
				Error:       err,
				ProcessTime: time.Since(startTime),
			}

		case <-ctx.Done():
			return
		}
	}
}

// ParseBlockByHash parses a block by its hash
func (p *Parser) ParseBlockByHash(ctx context.Context, blockHash string) (*types.ParsedBlock, error) {
	hash := common.HexToHash(blockHash)

	gethBlock, err := p.client.GetBlockByHash(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("failed to get block %s: %w", blockHash, err)
	}

	parsedBlock := types.NewParsedBlockFromGethBlock(gethBlock)

	transactions, err := p.parseBlockTransactions(ctx, gethBlock)
	if err != nil {
		return nil, fmt.Errorf("failed to parse transactions for block %s: %w", blockHash, err)
	}
	parsedBlock.Transactions = transactions

	return parsedBlock, nil
}

// GetLogsInRange retrieves and parses event logs within a block range
func (p *Parser) GetLogsInRange(ctx context.Context, startBlock, endBlock uint64, addresses []string, topics [][]string) ([]*types.ParsedLog, error) {
	// Convert string addresses to common.Address
	var filterAddresses []common.Address
	for _, addr := range addresses {
		if addr != "" {
			filterAddresses = append(filterAddresses, common.HexToAddress(addr))
		}
	}

	// Convert string topics to common.Hash
	var filterTopics [][]common.Hash
	for _, topicGroup := range topics {
		var topicHashes []common.Hash
		for _, topic := range topicGroup {
			if topic != "" {
				topicHashes = append(topicHashes, common.HexToHash(topic))
			}
		}
		if len(topicHashes) > 0 {
			filterTopics = append(filterTopics, topicHashes)
		}
	}

	query := ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(startBlock)),
		ToBlock:   big.NewInt(int64(endBlock)),
		Addresses: filterAddresses,
		Topics:    filterTopics,
	}

	gethLogs, err := p.client.GetLogs(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get logs: %w", err)
	}

	parsedLogs := make([]*types.ParsedLog, len(gethLogs))
	for i, gethLog := range gethLogs {
		parsedLogs[i] = types.NewParsedLogFromGethLog(&gethLog)
	}

	return parsedLogs, nil
}

// GetStats returns current parsing statistics
func (p *Parser) GetStats() types.ParsingStats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return *p.stats
}

// FilterTransactionsByAddress filters transactions by from/to address
func (p *Parser) FilterTransactionsByAddress(transactions []*types.ParsedTransaction, addresses []string) []*types.ParsedTransaction {
	if len(addresses) == 0 {
		return transactions
	}

	addressMap := make(map[string]bool)
	for _, addr := range addresses {
		addressMap[addr] = true
	}

	var filtered []*types.ParsedTransaction
	for _, tx := range transactions {
		if addressMap[tx.From] || (tx.To != nil && addressMap[*tx.To]) {
			filtered = append(filtered, tx)
		}
	}

	return filtered
}

// parseTransactionWithoutReceipt parses a transaction without fetching receipt data
func (p *Parser) parseTransactionWithoutReceipt(gethTx *gethTypes.Transaction, gethBlock *gethTypes.Block, txIndex uint) (*types.ParsedTransaction, error) {
	// Basic transaction parsing with safe field access
	var to *string
	if gethTx.To() != nil {
		toAddr := gethTx.To().Hex()
		to = &toAddr
	}

	// Safe from address extraction
	from := "unknown"
	txType := gethTx.Type()

	// Try different signer types for different transaction types
	if chainId := gethTx.ChainId(); chainId != nil && chainId.Sign() != 0 {
		// Try EIP-155 signer first
		if msg, err := gethTypes.NewEIP155Signer(chainId).Sender(gethTx); err == nil {
			from = msg.Hex()
		} else {
			// Fallback to other signers
			if msg, err := gethTypes.LatestSignerForChainID(chainId).Sender(gethTx); err == nil {
				from = msg.Hex()
			} else {
				signer := gethTypes.HomesteadSigner{}
				if msg, err := signer.Sender(gethTx); err == nil {
					from = msg.Hex()
				}
			}
		}
	}

	// Safe value access
	value := big.NewInt(0)
	if gethTx.Value() != nil {
		value = gethTx.Value()
	}

	// Safe gas price access
	gasPrice := big.NewInt(0)
	if gethTx.GasPrice() != nil {
		gasPrice = gethTx.GasPrice()
	}

	// Safe input data access
	inputData := ""
	if data := gethTx.Data(); data != nil {
		inputData = common.Bytes2Hex(data)
	}

	parsedTx := &types.ParsedTransaction{
		Hash:             gethTx.Hash().Hex(),
		BlockNumber:      gethBlock.NumberU64(),
		BlockHash:        gethBlock.Hash().Hex(),
		TransactionIndex: txIndex,
		From:             from,
		To:               to,
		Value:            value,
		Gas:              gethTx.Gas(),
		GasPrice:         gasPrice,
		InputData:        inputData,
		Nonce:            gethTx.Nonce(),
		Type:             txType,
		GasUsed:          0, // Not available without receipt
		Status:           2, // Use 2 to indicate "receipt not fetched"
	}

	// Safely add EIP-1559 fields for type 2 transactions
	if txType == 2 {
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Error accessing EIP-1559 fields for tx %s: %v", gethTx.Hash().Hex(), r)
				}
			}()

			if gasFeeCap := gethTx.GasFeeCap(); gasFeeCap != nil {
				parsedTx.MaxFeePerGas = gasFeeCap
			}
			if gasTipCap := gethTx.GasTipCap(); gasTipCap != nil {
				parsedTx.MaxPriorityFeePerGas = gasTipCap
			}
		}()
	}

	return parsedTx, nil
}

// GetContractCreations returns all contract creation transactions in a block range
func (p *Parser) GetContractCreations(ctx context.Context, startBlock, endBlock uint64) ([]*types.ParsedTransaction, error) {
	blocks, err := p.ParseBlockRange(ctx, startBlock, endBlock)
	if err != nil {
		return nil, err
	}

	var contractCreations []*types.ParsedTransaction
	for _, block := range blocks {
		for _, tx := range block.Transactions {
			if tx.To == nil || tx.ContractAddress != nil {
				contractCreations = append(contractCreations, tx)
			}
		}
	}

	return contractCreations, nil
}

// GetTransactionsByValue filters transactions by minimum value
func (p *Parser) GetTransactionsByValue(transactions []*types.ParsedTransaction, minValue *big.Int) []*types.ParsedTransaction {
	var filtered []*types.ParsedTransaction
	for _, tx := range transactions {
		if tx.Value.Cmp(minValue) >= 0 {
			filtered = append(filtered, tx)
		}
	}
	return filtered
}
