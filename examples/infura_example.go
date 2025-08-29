package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"eth-blockchain-parser/pkg/client"
	"eth-blockchain-parser/pkg/parser"
	"eth-blockchain-parser/pkg/types"
)

func main() {
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
		fmt.Println(`
Infura API Key Required!

To use this example:
1. Get your Infura API key from https://infura.io
2. Set one of these environment variables:
   - export INFURA_API_KEY="your-key-here"
   - export INFURA_PROJECT_ID="your-key-here"
   - export INFURA_KEY="your-key-here"

3. Optionally set the network:
   - export INFURA_NETWORK="mainnet"  (default)
   - export ETH_NETWORK="sepolia"     (alternative)

Supported networks: mainnet, sepolia, goerli, polygon-mainnet, arbitrum-mainnet

Your Infura "API Key" is actually your Project ID and looks like:
abc123def456789...

Example:
export INFURA_API_KEY="abc123def456..."
export INFURA_NETWORK="mainnet"
go run examples/infura_example.go
`)
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
	config.Workers = 2        // Lower workers for rate limiting
	config.BatchSize = 5      // Smaller batches
	config.IncludeLogs = true

	blockParser := parser.NewParser(ethClient, config)

	ctx := context.Background()

	// Get latest block number
	latest, err := ethClient.GetLatestBlockNumber(ctx)
	if err != nil {
		log.Fatalf("Failed to get latest block: %v", err)
	}

	fmt.Printf("Latest block: %d\n", latest)

	// Parse the last 5 blocks
	startBlock := latest - 4
	endBlock := latest

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

	filename := fmt.Sprintf("blocks_%d_%d.json", startBlock, endBlock)
	if err := os.WriteFile(filename, jsonData, 0644); err != nil {
		log.Fatalf("Failed to write file: %v", err)
	}

	fmt.Printf("Results saved to %s\n", filename)
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
