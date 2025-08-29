package client

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/trie"
)

// EthClient wraps the go-ethereum client with additional functionality
type EthClient struct {
	client      *ethclient.Client
	rpcClient   *rpc.Client
	nodeURL     string
	timeout     time.Duration
	retries     int
	isInfura    bool
	infuraConfig *InfuraConfig
	rateLimiter *time.Ticker // Simple rate limiting for Infura
}

// InfuraConfig holds Infura-specific configuration
type InfuraConfig struct {
	ProjectID string
	APIKey    string
	Network   string
	HTTPURL   string
	WSURL     string
}

// ConnectionConfig holds connection parameters
type ConnectionConfig struct {
	NodeURL         string
	WSNodeURL       string
	Timeout         time.Duration
	Retries         int
	UseInfura       bool
	InfuraAPIKey    string // This is the Project ID from Infura
	InfuraAPISecret string // Optional API Secret for paid plans
	InfuraNetwork   string
}

// NewEthClient creates a new Ethereum client wrapper
func NewEthClient(config ConnectionConfig) (*EthClient, error) {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.Retries == 0 {
		config.Retries = 3
	}

	client := &EthClient{
		nodeURL: config.NodeURL,
		timeout: config.Timeout,
		retries: config.Retries,
		isInfura: config.UseInfura,
	}

	// Setup Infura configuration if enabled
	if config.UseInfura {
		infuraConfig := &InfuraConfig{
			ProjectID: config.InfuraAPIKey, // API Key is actually the Project ID
			APIKey:    config.InfuraAPISecret, // API Secret (optional)
			Network:   config.InfuraNetwork,
			HTTPURL:   buildInfuraHTTPURL(config.InfuraNetwork, config.InfuraAPIKey, config.InfuraAPISecret),
			WSURL:     buildInfuraWSURL(config.InfuraNetwork, config.InfuraAPIKey, config.InfuraAPISecret),
		}
		client.infuraConfig = infuraConfig
		client.nodeURL = infuraConfig.HTTPURL
		
		// Set up rate limiting for Infura (2 requests per second to be very conservative)
		client.rateLimiter = time.NewTicker(500 * time.Millisecond)
		
		log.Printf("Using Infura API for network: %s", config.InfuraNetwork)
	}

	if err := client.connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to Ethereum node: %w", err)
	}

	return client, nil
}

// connect establishes connection to the Ethereum node
func (c *EthClient) connect() error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	rpcClient, err := rpc.DialContext(ctx, c.nodeURL)
	if err != nil {
		return fmt.Errorf("failed to connect to RPC: %w", err)
	}

	c.rpcClient = rpcClient
	c.client = ethclient.NewClient(rpcClient)

	// Test the connection with rate limiting
	c.waitForRateLimit()
	if _, err := c.client.NetworkID(ctx); err != nil {
		c.rpcClient.Close()
		return fmt.Errorf("failed to verify connection: %w", err)
	}

	log.Printf("Connected to Ethereum node at %s", c.nodeURL)
	return nil
}

// Close closes the connection to the Ethereum node
func (c *EthClient) Close() {
	if c.rateLimiter != nil {
		c.rateLimiter.Stop()
	}
	if c.rpcClient != nil {
		c.rpcClient.Close()
	}
}

// GetLatestBlockNumber returns the latest block number with rate limit handling
func (c *EthClient) GetLatestBlockNumber(ctx context.Context) (uint64, error) {
	result, err := c.executeWithRetry(func() (interface{}, error) {
		header, err := c.client.HeaderByNumber(ctx, nil)
		if err != nil {
			return nil, err
		}
		return header.Number.Uint64(), nil
	})
	
	if err != nil {
		return 0, err
	}
	
	return result.(uint64), nil
}

// GetBlockByNumber retrieves a block by its number with error handling for unsupported transaction types
func (c *EthClient) GetBlockByNumber(ctx context.Context, blockNumber uint64) (*types.Block, error) {
	result, err := c.executeWithRetry(func() (interface{}, error) {
		// First try the standard method
		block, err := c.client.BlockByNumber(ctx, big.NewInt(int64(blockNumber)))
		if err == nil {
			return block, nil
		}
		
		// If we get a "transaction type not supported" error, try to reconstruct the block
		if strings.Contains(err.Error(), "transaction type not supported") {
			log.Printf("Block %d contains unsupported transaction types, attempting to reconstruct with supported transactions", blockNumber)
			return c.getBlockWithFilteredTransactions(ctx, blockNumber)
		}
		
		return nil, err
	})
	
	if err != nil {
		return nil, err
	}
	
	return result.(*types.Block), nil
}

// GetBlockByHash retrieves a block by its hash
func (c *EthClient) GetBlockByHash(ctx context.Context, blockHash common.Hash) (*types.Block, error) {
	c.waitForRateLimit()
	return c.client.BlockByHash(ctx, blockHash)
}

// GetTransactionReceipt retrieves transaction receipt
func (c *EthClient) GetTransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error) {
	c.waitForRateLimit()
	return c.client.TransactionReceipt(ctx, txHash)
}

// GetTransactionReceiptsBatch retrieves multiple transaction receipts in a batch with rate limit handling
func (c *EthClient) GetTransactionReceiptsBatch(ctx context.Context, txHashes []common.Hash) ([]*types.Receipt, error) {
	result, err := c.executeWithRetry(func() (interface{}, error) {
		receipts := make([]*types.Receipt, len(txHashes))
		
		// Create batch request
		batch := make([]rpc.BatchElem, len(txHashes))
		for i, txHash := range txHashes {
			batch[i] = rpc.BatchElem{
				Method: "eth_getTransactionReceipt",
				Args:   []interface{}{txHash.Hex()},
				Result: &receipts[i],
			}
		}

		if err := c.rpcClient.BatchCallContext(ctx, batch); err != nil {
			return nil, err
		}
		
		// Check for individual errors
		for i, elem := range batch {
			if elem.Error != nil {
				log.Printf("Error getting receipt for tx %s: %v", txHashes[i].Hex(), elem.Error)
				receipts[i] = nil
			}
		}
		
		return receipts, nil
	})
	
	if err != nil {
		return nil, err
	}
	
	return result.([]*types.Receipt), nil
}

// GetLogs retrieves event logs based on filter criteria
func (c *EthClient) GetLogs(ctx context.Context, query ethereum.FilterQuery) ([]types.Log, error) {
	c.waitForRateLimit()
	return c.client.FilterLogs(ctx, query)
}

// GetNetworkID returns the network/chain ID
func (c *EthClient) GetNetworkID(ctx context.Context) (*big.Int, error) {
	c.waitForRateLimit()
	return c.client.NetworkID(ctx)
}

// GetBalance returns the balance of an account at a specific block
func (c *EthClient) GetBalance(ctx context.Context, account common.Address, blockNumber *big.Int) (*big.Int, error) {
	c.waitForRateLimit()
	return c.client.BalanceAt(ctx, account, blockNumber)
}

// GetCode returns the contract code at a specific address and block
func (c *EthClient) GetCode(ctx context.Context, contract common.Address, blockNumber *big.Int) ([]byte, error) {
	c.waitForRateLimit()
	return c.client.CodeAt(ctx, contract, blockNumber)
}

// IsConnected checks if the client is connected to the node
func (c *EthClient) IsConnected(ctx context.Context) bool {
	_, err := c.client.NetworkID(ctx)
	return err == nil
}

// Reconnect attempts to reconnect to the Ethereum node
func (c *EthClient) Reconnect() error {
	c.Close()
	return c.connect()
}

// executeWithRetry executes a function with automatic retry on connection errors
func (c *EthClient) executeWithRetry(fn func() (interface{}, error)) (interface{}, error) {
	var result interface{}
	var err error
	
	for attempt := 0; attempt <= c.retries; attempt++ {
		if attempt > 0 {
			// Wait before retry with exponential backoff
			waitTime := time.Duration(attempt) * time.Second
			log.Printf("Retrying in %v (attempt %d/%d)", waitTime, attempt, c.retries)
			time.Sleep(waitTime)
			
			// Try to reconnect
			if err := c.Reconnect(); err != nil {
				log.Printf("Failed to reconnect: %v", err)
				continue
			}
		}
		
		// Apply rate limiting for Infura
		c.waitForRateLimit()
		
		result, err = fn()
		if err == nil {
			return result, nil
		}
		
		// Check for rate limit errors and handle them specially
		if c.isRateLimitError(err) {
			waitTime := c.calculateRateLimitBackoff(attempt)
			log.Printf("Rate limit exceeded, waiting %v before retry (attempt %d/%d)", waitTime, attempt+1, c.retries+1)
			time.Sleep(waitTime)
			continue
		}
		
		log.Printf("Attempt %d failed: %v", attempt+1, err)
	}
	
	return result, fmt.Errorf("failed after %d attempts: %w", c.retries+1, err)
}

// isRateLimitError checks if the error is a rate limit error
func (c *EthClient) isRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	errorStr := err.Error()
	return strings.Contains(errorStr, "429") || 
		   strings.Contains(errorStr, "Too Many Requests") ||
		   strings.Contains(errorStr, "rate limit") ||
		   strings.Contains(errorStr, "exceeded")
}

// calculateRateLimitBackoff calculates exponential backoff for rate limit errors
func (c *EthClient) calculateRateLimitBackoff(attempt int) time.Duration {
	// Start with 1 second, double each attempt, max 60 seconds
	baseDelay := time.Second
	maxDelay := 60 * time.Second
	
	delay := baseDelay * time.Duration(1<<uint(attempt))
	if delay > maxDelay {
		delay = maxDelay
	}
	
	return delay
}

// buildInfuraHTTPURL constructs the Infura HTTP URL
func buildInfuraHTTPURL(network, projectID, apiKey string) string {
	baseURL := fmt.Sprintf("https://%s.infura.io/v3/%s", network, projectID)
	if apiKey != "" {
		return fmt.Sprintf("%s/%s", baseURL, apiKey)
	}
	return baseURL
}

// buildInfuraWSURL constructs the Infura WebSocket URL
func buildInfuraWSURL(network, projectID, apiKey string) string {
	baseURL := fmt.Sprintf("wss://%s.infura.io/ws/v3/%s", network, projectID)
	if apiKey != "" {
		return fmt.Sprintf("%s/%s", baseURL, apiKey)
	}
	return baseURL
}

// waitForRateLimit implements rate limiting for Infura requests
func (c *EthClient) waitForRateLimit() {
	if c.isInfura && c.rateLimiter != nil {
		<-c.rateLimiter.C
	}
}

// getBlockWithFilteredTransactions attempts to get block data using raw RPC calls to handle unsupported transaction types
func (c *EthClient) getBlockWithFilteredTransactions(ctx context.Context, blockNumber uint64) (*types.Block, error) {
	c.waitForRateLimit()
	
	// Use raw RPC call to get block with transactions, but with error recovery
	var result map[string]interface{}
	err := c.rpcClient.CallContext(ctx, &result, "eth_getBlockByNumber", fmt.Sprintf("0x%x", blockNumber), true)
	if err != nil {
		log.Printf("Raw RPC call failed for block %d: %v", blockNumber, err)
		return c.getBlockWithHeaderOnly(ctx, blockNumber)
	}
	
	if result == nil {
		return nil, fmt.Errorf("block %d not found", blockNumber)
	}
	
	// Extract block header information
	header, err := c.parseBlockHeader(result)
	if err != nil {
		log.Printf("Failed to parse block header for block %d: %v", blockNumber, err)
		return c.getBlockWithHeaderOnly(ctx, blockNumber)
	}
	
	// Extract transactions with error handling
	txs, skipped := c.parseBlockTransactions(result, blockNumber)
	
	log.Printf("Successfully parsed block %d with %d transactions (%d skipped due to unsupported types)", blockNumber, len(txs), skipped)
	
	// Create block with the parsed transactions
	emptyUncles := make([]*types.Header, 0)
	// Use the default hasher to avoid nil pointer dereference in DeriveSha
	hasher := trie.NewStackTrie(nil)
	block := types.NewBlock(header, txs, emptyUncles, nil, hasher)
	
	return block, nil
}

// getBlockWithHeaderOnly creates a block with only header info when transaction parsing fails
func (c *EthClient) getBlockWithHeaderOnly(ctx context.Context, blockNumber uint64) (*types.Block, error) {
	c.waitForRateLimit()
	
	// Get the block header
	header, err := c.client.HeaderByNumber(ctx, big.NewInt(int64(blockNumber)))
	if err != nil {
		return nil, fmt.Errorf("failed to get header for block %d: %w", blockNumber, err)
	}
	
	// Create a block with empty transactions and empty uncles
	// This allows us to continue parsing even when transactions have unsupported types
	emptyTxs := make([]*types.Transaction, 0)
	emptyUncles := make([]*types.Header, 0)
	
	// Create a new block with the header and empty transaction/uncle lists
	// Use the default hasher to avoid nil pointer dereference in DeriveSha
	hasher := trie.NewStackTrie(nil)
	block := types.NewBlock(header, emptyTxs, emptyUncles, nil, hasher)
	
	log.Printf("Created fallback block %d with header only (transactions skipped due to unsupported types)", blockNumber)
	
	return block, nil
}

// NewInfuraClient creates a new Ethereum client specifically for Infura
// apiKey parameter is your Infura Project ID (what Infura calls "API Key")
// apiSecret parameter is optional and only needed for paid plans
func NewInfuraClient(apiKey, apiSecret, network string) (*EthClient, error) {
	config := ConnectionConfig{
		UseInfura:       true,
		InfuraAPIKey:    apiKey, // This is the Project ID
		InfuraAPISecret: apiSecret,
		InfuraNetwork:   network,
		Timeout:         30 * time.Second,
		Retries:         3,
	}
	
	config.NodeURL = buildInfuraHTTPURL(network, apiKey, apiSecret)
	config.WSNodeURL = buildInfuraWSURL(network, apiKey, apiSecret)
	
	return NewEthClient(config)
}

// NewInfuraClientSimple creates a new Ethereum client for Infura with just the API key
func NewInfuraClientSimple(apiKey, network string) (*EthClient, error) {
	return NewInfuraClient(apiKey, "", network)
}

// GetInfuraRateLimitInfo returns rate limit information for Infura
func (c *EthClient) GetInfuraRateLimitInfo() map[string]interface{} {
	info := make(map[string]interface{})
	
	if !c.isInfura {
		info["is_infura"] = false
		return info
	}
	
	info["is_infura"] = true
	info["network"] = c.infuraConfig.Network
	info["project_id"] = c.infuraConfig.ProjectID
	info["has_api_key"] = c.infuraConfig.APIKey != ""
	info["http_url"] = c.infuraConfig.HTTPURL
	info["ws_url"] = c.infuraConfig.WSURL
	
	return info
}

// parseBlockHeader parses block header from raw RPC response
func (c *EthClient) parseBlockHeader(result map[string]interface{}) (*types.Header, error) {
	// Convert the result to JSON and back to get proper type handling
	jsonData, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal block data: %w", err)
	}
	
	// Parse using go-ethereum's JSON unmarshaling
	var header types.Header
	err = header.UnmarshalJSON(jsonData)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal block header: %w", err)
	}
	
	return &header, nil
}

// parseBlockTransactions parses transactions from raw RPC response, skipping unsupported types
func (c *EthClient) parseBlockTransactions(result map[string]interface{}, blockNumber uint64) ([]*types.Transaction, int) {
	txsData, ok := result["transactions"].([]interface{})
	if !ok {
		log.Printf("No transactions field found in block %d", blockNumber)
		return make([]*types.Transaction, 0), 0
	}
	
	txs := make([]*types.Transaction, 0, len(txsData))
	skipped := 0
	
	for i, txData := range txsData {
		txMap, ok := txData.(map[string]interface{})
		if !ok {
			log.Printf("Invalid transaction data at index %d in block %d", i, blockNumber)
			skipped++
			continue
		}
		
		// Try to parse the transaction
		tx, err := c.parseTransaction(txMap)
		if err != nil {
			log.Printf("Failed to parse transaction at index %d in block %d: %v", i, blockNumber, err)
			skipped++
			continue
		}
		
		txs = append(txs, tx)
	}
	
	return txs, skipped
}

// parseTransaction parses a single transaction from raw RPC data
func (c *EthClient) parseTransaction(txMap map[string]interface{}) (*types.Transaction, error) {
	// Convert to JSON and parse using go-ethereum's JSON unmarshaling
	jsonData, err := json.Marshal(txMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal transaction data: %w", err)
	}
	
	var tx types.Transaction
	err = tx.UnmarshalJSON(jsonData)
	if err != nil {
		// If parsing fails due to unsupported transaction type, try to extract basic info
		if strings.Contains(err.Error(), "transaction type not supported") || strings.Contains(err.Error(), "unsupported transaction type") {
			return c.createFallbackTransaction(txMap)
		}
		return nil, err
	}
	
	return &tx, nil
}

// createFallbackTransaction creates a basic transaction object for unsupported transaction types
func (c *EthClient) createFallbackTransaction(txMap map[string]interface{}) (*types.Transaction, error) {
	// Extract basic fields that are common to all transaction types
	hash, _ := txMap["hash"].(string)
	from, _ := txMap["from"].(string)
	to, _ := txMap["to"].(string)
	value, _ := txMap["value"].(string)
	gas, _ := txMap["gas"].(string)
	gasPrice, _ := txMap["gasPrice"].(string)
	nonce, _ := txMap["nonce"].(string)
	
	// Convert hex strings to appropriate types
	nonceBig := new(big.Int)
	gasBig := new(big.Int)
	gasPriceBig := new(big.Int)
	valueBig := new(big.Int)
	
	if nonce != "" {
		nonceBig.SetString(strings.TrimPrefix(nonce, "0x"), 16)
	}
	
	if gas != "" {
		gasBig.SetString(strings.TrimPrefix(gas, "0x"), 16)
	}
	
	if gasPrice != "" {
		gasPriceBig.SetString(strings.TrimPrefix(gasPrice, "0x"), 16)
	}
	
	if value != "" {
		valueBig.SetString(strings.TrimPrefix(value, "0x"), 16)
	}
	
	// Create a legacy transaction (type 0) as fallback
	var toAddr *common.Address
	if to != "" {
		addr := common.HexToAddress(to)
		toAddr = &addr
	}
	
	// Create legacy transaction with available data
	legacyTx := &types.LegacyTx{
		Nonce:    nonceBig.Uint64(),
		GasPrice: gasPriceBig,
		Gas:      gasBig.Uint64(),
		To:       toAddr,
		Value:    valueBig,
		Data:     []byte{}, // Empty data for safety
	}
	
	tx := types.NewTx(legacyTx)
	
	fmt.Printf("Created fallback transaction: hash=%s, from=%s, to=%s, value=%s ETH (unsupported type)\n", 
		hash, from, 
		func() string {
			if to != "" {
				return to
			}
			return "CONTRACT_CREATION"
		}(),
		fmt.Sprintf("%.6f", float64(valueBig.Uint64())/1e18))
	
	return tx, nil
}
