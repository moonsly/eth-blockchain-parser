package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"syscall"

	"eth-blockchain-parser/pkg/client"
	"eth-blockchain-parser/pkg/database"
	"eth-blockchain-parser/pkg/filtering"
	"eth-blockchain-parser/pkg/parser"
	"eth-blockchain-parser/pkg/types"
)

func main() {
	// check lock file
	lockFilePath := "/tmp/eth_parser.lock"

	// Open the lock file (create if it doesn't exist)
	f, err := os.OpenFile(lockFilePath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		log.Fatalf("Failed to open lock file: %v", err)
	}
	defer f.Close() // Ensure the file is closed on exit

	// Attempt to acquire an exclusive, non-blocking lock
	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		if err == syscall.EWOULDBLOCK {
			fmt.Println("Another instance of the script is already running. Exiting.")
			os.Exit(1) // Exit if lock cannot be acquired
		}
		log.Fatalf("Failed to acquire file lock: %v", err)
	}
	// lock acquired - continue, unlock in defer

	fmt.Println("Lock acquired. Running script...")
	// Initialize database
	logger := log.New(os.Stdout, "[ETH-PARSER-DB] ", log.LstdFlags|log.Lshortfile)
	logger.Println("Initializing database...")

	dbConfig := database.DefaultConfig("./blockchain.db")
	dbManager, err := database.NewDatabaseManager(dbConfig, logger)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer dbManager.Close()

	ctx := context.Background()
	// Initialize repositories
	txRepo := database.NewTransactionRepository(dbManager, logger)
	addressRepo := database.NewAddressRepository(dbManager, logger)

	// check if main tables exists
	_, err1 := addressRepo.GetWatched(ctx)
	txs1, err2 := txRepo.GetByBlockNumber(ctx, 123)
	if len(txs1) > 0 {
		logger.Println("current txs1[0]", txs1[0])
	}
	// create schema if no tables
	if err1 != nil && err2 != nil {
		schema := database.NewSchema(logger)
		db, err := dbManager.DB()
		if err != nil {
			logger.Fatalf("Failed to get database connection: %v", err)
		}
		if err := schema.CreateAllTables(db); err != nil {
			logger.Fatalf("Failed to create tables: %v", err)
		}
	}

	//logger.Fatalf("BYE")

	// Get Infura API key from environment variables (supports multiple env var names)
	infuraAPIKey := getInfuraAPIKey()

	// Get network from environment variable (defaults to mainnet)
	network := os.Getenv("INFURA_NETWORK")
	if network == "" {
		network = os.Getenv("ETH_NETWORK")
	}
	if network == "" {
		network = "mainnet" // Default to mainnet
	}

	if infuraAPIKey == "YOUR_INFURA_API_KEY_HERE" || infuraAPIKey == "" {
		fmt.Println(`Infura API Key Required!

To use this parser with Infura:
1. Get your Infura API key from https://infura.io
2. Set one of these environment variables:
   - export INFURA_API_KEY="your-key-here"
3. Optionally set the network:
   - export INFURA_NETWORK="mainnet"  (default)
   - export ETH_NETWORK="sepolia"     (alternative)

Supported networks: mainnet, sepolia, goerli, polygon-mainnet, arbitrum-mainnet

Your Infura "API Key" usually looks like: abc123def456789...`)
		return
	}

	log.Printf("Using Infura API Key: %s... (network: %s)", infuraAPIKey[:8], network)

	// Create Infura client
	ethClient, err := client.NewInfuraClientSimple(infuraAPIKey, network)
	if err != nil {
		log.Fatalf("Failed to create Infura client: %v", err)
	}
	defer ethClient.Close()

	// Show connection info
	info := ethClient.GetInfuraRateLimitInfo()
	fmt.Printf("Connected to Infura: %+v\n", info)

	// Create parser with Infura-optimized config
	config := types.InfuraConfigSimple(infuraAPIKey, network)

	// CLI flags
	initw := flag.Bool("initw", false, "recreate WhaleAddreses in DB and exit")
	flag.Parse()
	if *initw {
		fmt.Printf("Recreating WhaleAddress in DB mode: %v\n", *initw)
		err := initWhales(ctx, addressRepo, config.WhalesAddr)
		if err != nil {
			log.Fatalf("Failed recreate initw %s", err)
		} else {
			log.Fatalf("Created WhaleAddresses OK")
		}
	}

	blockParser := parser.NewParser(ethClient, config)

	// Get latest block number
	latest, err := ethClient.GetLatestBlockNumber(ctx)
	if err != nil {
		log.Fatalf("Failed to get latest block: %v", err)
	}

	fmt.Printf("Latest block: %d\n", latest)

	// Parse blocks from lastBlock in file
	startBlock := filtering.ReadLastBlock(config.LastBlockPath) + 1
	endBlock := latest
	// если сервис долго простаивал - парсим только последние config.MaxBlockDelta блоков от latest
	// иначе долго будем догонять latest block, пропустим актуальные крупные ЕТН транзакции
	if endBlock-startBlock > config.MaxBlockDelta {
		startBlock = latest - config.MaxBlockDelta
	}

	fmt.Printf("Parsing blocks %d to %d...\n", startBlock, endBlock)

	blocks, err := blockParser.ParseBlockRange(ctx, startBlock, endBlock)
	if err != nil {
		log.Fatalf("Failed to parse blocks: %v", err)
	}

	// Output summary
	fmt.Printf("\n=== Parsing Results ===\n")
	totalTransactions := 0
	totalLogs := 0

	for _, block := range blocks {
		totalTransactions += len(block.Transactions)
		for _, tx := range block.Transactions {
			if tx.Logs != nil {
				totalLogs += len(tx.Logs)
			}
		}
		fmt.Printf("Block %d: %d transactions, %d gas used\n",
			block.Number, len(block.Transactions), block.GasUsed)
	}

	fmt.Printf("\nTotal: %d blocks, %d transactions, %d logs\n",
		len(blocks), totalTransactions, totalLogs)

	// Show parsing stats
	stats := blockParser.GetStats()
	fmt.Printf("Processing time: %v\n", stats.TotalDuration)

	// Save to file
	jsonData, err := json.MarshalIndent(blocks, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal JSON: %v", err)
	}

	if config.DumpJsonFile {
		filename := fmt.Sprintf("blocks_%d_%d.json", startBlock, endBlock)
		if err := os.WriteFile(filename, jsonData, 0644); err != nil {
			log.Fatalf("Failed to write file: %v", err)
		}
		fmt.Printf("Results saved to %s\n", filename)
	}

	lastBlock := blocks[len(blocks)-1].Number
	fmt.Printf("Last block parsed: %d\n", lastBlock)
	filtering.WriteLastBlock(config.LastBlockPath, lastBlock)

	tx_filtered := filtering.ParseWhaleTransactions(ctx, blocks, config.WhalesAddr, config.MinETHValue, addressRepo)
	fmt.Println("TX filtered", tx_filtered)

	whale_txn := filtering.TransformTxsToCsv(tx_filtered, config.WhalesAddr)
	fmt.Println(whale_txn)
	filtering.AppendCSV(config.CsvPath, whale_txn)

	err = txRepo.BatchInsert(ctx, tx_filtered)
	if err != nil {
		logger.Fatalf("Error inserting to db:%s", err)
	}
}

func initWhales(ctx context.Context, ar *database.AddressRepository, whales map[string]string) error {
	err := ar.DeleteAll(ctx)
	if err != nil {
		return fmt.Errorf("failed to insert address: %w", err)
	}
	keys := make([]string, 0, len(whales))
	for k := range whales {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	addrs := make([]*database.WhaleAddress, 0, len(whales))
	for _, el := range keys {
		lbl := whales[el]
		w_addr := database.WhaleAddress{Address: strings.ToLower(el), Label: &lbl}
		addrs = append(addrs, &w_addr)
	}

	err2 := ar.BatchInsert(ctx, addrs)
	return err2
}

// getInfuraAPIKey tries multiple environment variable names to get the Infura API key
func getInfuraAPIKey() string {
	// Try common environment variable names
	envVars := []string{
		"INFURA_API_KEY",
		"INFURA_PROJECT_ID",
		"INFURA_KEY",
		"INFURA_ID",
	}

	for _, envVar := range envVars {
		if key := os.Getenv(envVar); key != "" {
			log.Printf("Found API key in %s environment variable", envVar)
			return key
		}
	}

	// Fallback to hardcoded value
	return "YOUR_INFURA_API_KEY_HERE"
}
