package main

import (
	"context"
	"fmt"
	"log"

	"eth-blockchain-parser/pkg/client"
	"eth-blockchain-parser/pkg/parser"
	"eth-blockchain-parser/pkg/types"
)

func runSimpleExample() {
	// Replace this with your actual Infura API key (Project ID)
	// You can get this from https://infura.io after creating a project
	infuraAPIKey := "af0eb78c2b874cb2a2df3515af9182fa"

	if infuraAPIKey == "YOUR_INFURA_API_KEY_HERE" {
		fmt.Println(`
Please replace "YOUR_INFURA_API_KEY_HERE" with your actual Infura API key.

To get your API key:
1. Go to https://infura.io
2. Sign up and create a new project  
3. Copy the "API Key" from your dashboard (this is actually your Project ID)

Example: infuraAPIKey := "abc123def456789..."
`)
		return
	}

	// Create Infura client with just your API key
	ethClient, err := client.NewInfuraClientSimple(infuraAPIKey, "mainnet")
	if err != nil {
		log.Fatalf("Failed to connect to Infura: %v", err)
	}
	defer ethClient.Close()

	// Show connection info
	fmt.Println("✅ Connected to Infura successfully!")

	ctx := context.Background()

	// Get latest block number
	latest, err := ethClient.GetLatestBlockNumber(ctx)
	if err != nil {
		log.Fatalf("Failed to get latest block: %v", err)
	}
	fmt.Printf("📊 Latest block number: %d\n", latest)

	// Create parser
	config := types.InfuraConfigSimple(infuraAPIKey, "mainnet")
	config.Workers = 1         // Single worker to be gentle with rate limits
	config.IncludeLogs = false // Disable logs for faster parsing

	blockParser := parser.NewParser(ethClient, config)

	// Parse the latest block
	fmt.Printf("🔍 Parsing latest block %d...\n", latest)
	block, err := blockParser.ParseSingleBlock(ctx, latest)
	if err != nil {
		log.Fatalf("Failed to parse block: %v", err)
	}

	// Display summary
	fmt.Printf(`
📦 Block Summary:
   Number: %d
   Hash: %s
   Transactions: %d
   Gas Used: %d / %d (%.1f%%)
   Miner: %s
   Timestamp: %s
`,
		block.Number,
		block.Hash[:10]+"...", // Show first 10 chars of hash
		len(block.Transactions),
		block.GasUsed,
		block.GasLimit,
		float64(block.GasUsed)/float64(block.GasLimit)*100,
		block.Miner[:10]+"...", // Show first 10 chars of miner address
		block.Timestamp.Format("2006-01-02 15:04:05"),
	)

	// Show top 5 transactions by value
	if len(block.Transactions) > 0 {
		fmt.Printf("\n💰 Top transactions by value:\n")
		count := 5
		if len(block.Transactions) < count {
			count = len(block.Transactions)
		}

		for i := 0; i < count; i++ {
			tx := block.Transactions[i]
			ethValue := float64(tx.Value.Uint64()) / 1e18
			fmt.Printf("   %d. %s... → %s... (%.4f ETH)\n",
				i+1,
				tx.From[:10],
				func() string {
					if tx.To != nil {
						return (*tx.To)[:10]
					}
					return "CONTRACT"
				}(),
				ethValue,
			)
		}
	}

	fmt.Println("\n✨ Done! This demonstrates basic Infura connectivity with just your API key.")
}

func main() {
	runSimpleExample()
}
