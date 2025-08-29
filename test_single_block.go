package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"eth-blockchain-parser/pkg/client"
	"eth-blockchain-parser/pkg/parser"
	"eth-blockchain-parser/pkg/types"
)

func main() {
	// Get API key from environment or use default
	apiKey := os.Getenv("INFURA_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("INFURA_PROJECT_ID")
	}
	if apiKey == "" {
		// Use the API key from simple_infura.go as fallback
		apiKey = "af0eb78c2b874cb2a2df3515af9182fa"
	}

	network := os.Getenv("INFURA_NETWORK")
	if network == "" {
		network = "mainnet"
	}

	// Create Infura client
	ethClient, err := client.NewInfuraClientSimple(apiKey, network)
	if err != nil {
		log.Fatalf("Failed to connect to Infura: %v", err)
	}
	defer ethClient.Close()

	fmt.Println("✅ Connected to Infura successfully!")

	ctx := context.Background()

	// Create parser
	config := types.InfuraConfigSimple(apiKey, network)
	config.Workers = 1
	config.IncludeLogs = false

	blockParser := parser.NewParser(ethClient, config)

	// Test one problematic block
	blockNum := uint64(23249240)

	fmt.Printf("🔍 Testing block %d...\n", blockNum)
	
	block, err := blockParser.ParseSingleBlock(ctx, blockNum)
	if err != nil {
		log.Fatalf("Failed to parse block %d: %v", blockNum, err)
	}

	fmt.Printf("📦 Block %d:\n", blockNum)
	fmt.Printf("   Hash: %s\n", block.Hash[:10]+"...")
	fmt.Printf("   Transactions: %d\n", len(block.Transactions))
	fmt.Printf("   Gas Used: %d\n", block.GasUsed)
	fmt.Printf("   Gas Limit: %d\n", block.GasLimit)
	fmt.Printf("   Miner: %s\n", block.Miner[:10]+"...")

	if len(block.Transactions) > 0 {
		fmt.Printf("   First TX Hash: %s\n", block.Transactions[0].Hash[:10]+"...")
		fmt.Printf("   First TX Type: %d\n", block.Transactions[0].Type)
		fmt.Printf("   First TX Value: %s ETH\n", fmt.Sprintf("%.6f", float64(block.Transactions[0].Value.Uint64())/1e18))
	} else {
		fmt.Printf("   ⚠️  No transactions found (may be due to unsupported transaction types)\n")
	}

	fmt.Println("✨ Test completed!")
}
