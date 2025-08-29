package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"eth-blockchain-parser/pkg/client"
	"eth-blockchain-parser/pkg/parser"
	"eth-blockchain-parser/pkg/types"
)

func main() {
	var (
		infuraAPIKey  = flag.String("infura-api-key", "", "Your Infura API Key (Project ID)")
		network       = flag.String("network", "mainnet", "Network (mainnet, goerli, sepolia, etc.)")
		startBlock    = flag.String("start", "latest-10", "Start block (number or 'latest-N')")
		endBlock      = flag.String("end", "latest", "End block (number or 'latest')")
		outputFile    = flag.String("output", "", "Output file (default: stdout)")
		includeLogs   = flag.Bool("logs", true, "Include transaction logs")
		filterAddr    = flag.String("filter-address", "", "Filter by address")
		workers       = flag.Int("workers", 2, "Number of workers (keep low for Infura)")
		batchSize     = flag.Int("batch-size", 5, "Batch size (keep small for Infura)")
	)
	flag.Parse()

	// Get API key from environment if not provided
	apiKey := *infuraAPIKey
	if apiKey == "" {
		apiKey = os.Getenv("INFURA_API_KEY")
	}

	if apiKey == "" {
		fmt.Println(`
Infura API Key required!

Usage:
  1. Set environment variable: export INFURA_API_KEY="your-key-here"
  2. Or use flag: -infura-api-key="your-key-here"

Get your API key from: https://infura.io
(What Infura calls "API Key" is actually your Project ID)

Examples:
  # Parse last 10 blocks
  ./infura-parser -infura-api-key="abc123..." 

  # Parse specific range
  ./infura-parser -infura-api-key="abc123..." -start=18000000 -end=18000010

  # Filter by address
  ./infura-parser -infura-api-key="abc123..." -filter-address="0x742d35Cc6534C0532925a3b8D"

  # Use different network
  ./infura-parser -infura-api-key="abc123..." -network="sepolia"
`)
		os.Exit(1)
	}

	// Create Infura client
	ethClient, err := client.NewInfuraClientSimple(apiKey, *network)
	if err != nil {
		log.Fatalf("Failed to create Infura client: %v", err)
	}
	defer ethClient.Close()

	// Show connection info
	info := ethClient.GetInfuraRateLimitInfo()
	log.Printf("Connected to Infura: network=%s, project_id=%s", 
		info["network"], info["project_id"])

	// Create parser config optimized for Infura
	config := types.InfuraConfigSimple(apiKey, *network)
	config.Workers = *workers
	config.BatchSize = uint64(*batchSize)
	config.IncludeLogs = *includeLogs

	blockParser := parser.NewParser(ethClient, config)

	ctx := context.Background()

	// Parse block range
	start, end, err := parseBlockRange(ctx, ethClient, *startBlock, *endBlock)
	if err != nil {
		log.Fatalf("Failed to parse block range: %v", err)
	}

	log.Printf("Parsing blocks from %d to %d", start, end)

	blocks, err := blockParser.ParseBlockRange(ctx, start, end)
	if err != nil {
		log.Fatalf("Failed to parse blocks: %v", err)
	}

	// Apply address filter if specified
	if *filterAddr != "" {
		addresses := strings.Split(*filterAddr, ",")
		for _, block := range blocks {
			block.Transactions = blockParser.FilterTransactionsByAddress(block.Transactions, addresses)
		}
		log.Printf("Filtered by addresses: %v", addresses)
	}

	// Output results
	jsonData, err := json.MarshalIndent(blocks, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal JSON: %v", err)
	}

	if *outputFile != "" {
		if err := os.WriteFile(*outputFile, jsonData, 0644); err != nil {
			log.Fatalf("Failed to write output file: %v", err)
		}
		log.Printf("Results written to %s", *outputFile)
	} else {
		fmt.Println(string(jsonData))
	}

	// Show summary
	stats := blockParser.GetStats()
	log.Printf("Processed %d blocks, %d transactions in %v", 
		stats.BlocksParsed, stats.TransactionsParsed, stats.TotalDuration)
}

// parseBlockRange parses block range from string arguments
func parseBlockRange(ctx context.Context, ethClient *client.EthClient, startStr, endStr string) (uint64, uint64, error) {
	// Get latest block for reference
	latest, err := ethClient.GetLatestBlockNumber(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get latest block: %w", err)
	}

	// Parse start block
	var start uint64
	if startStr == "latest" {
		start = latest
	} else if strings.HasPrefix(startStr, "latest-") {
		offset, parseErr := strconv.ParseUint(startStr[7:], 10, 64)
		if parseErr != nil {
			return 0, 0, fmt.Errorf("invalid start block format: %w", parseErr)
		}
		if offset > latest {
			start = 0
		} else {
			start = latest - offset
		}
	} else {
		start, err = strconv.ParseUint(startStr, 10, 64)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid start block: %w", err)
		}
	}

	// Parse end block
	var end uint64
	if endStr == "latest" {
		end = latest
	} else if strings.HasPrefix(endStr, "latest-") {
		offset, parseErr := strconv.ParseUint(endStr[7:], 10, 64)
		if parseErr != nil {
			return 0, 0, fmt.Errorf("invalid end block format: %w", parseErr)
		}
		if offset > latest {
			end = 0
		} else {
			end = latest - offset
		}
	} else {
		end, err = strconv.ParseUint(endStr, 10, 64)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid end block: %w", err)
		}
	}

	if start > end {
		return 0, 0, fmt.Errorf("start block (%d) cannot be greater than end block (%d)", start, end)
	}

	return start, end, nil
}
