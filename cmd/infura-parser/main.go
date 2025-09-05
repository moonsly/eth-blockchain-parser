package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"strconv"
	"strings"
	"time"

	"eth-blockchain-parser/pkg/client"
	"eth-blockchain-parser/pkg/parser"
	"eth-blockchain-parser/pkg/types"

	"github.com/shopspring/decimal"
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

	blockParser := parser.NewParser(ethClient, config)

	ctx := context.Background()

	// Get latest block number
	latest, err := ethClient.GetLatestBlockNumber(ctx)
	if err != nil {
		log.Fatalf("Failed to get latest block: %v", err)
	}

	fmt.Printf("Latest block: %d\n", latest)

	// Parse blocks from lastBlock in file
	startBlock := readLastBlock(config.LastBlockPath) + 1
	endBlock := latest
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

	filename := fmt.Sprintf("blocks_%d_%d.json", startBlock, endBlock)
	if err := os.WriteFile(filename, jsonData, 0644); err != nil {
		log.Fatalf("Failed to write file: %v", err)
	}
	fmt.Printf("Results saved to %s\n", filename)

	lastBlock := blocks[len(blocks)-1].Number
	fmt.Printf("Last block parsed: %d\n", lastBlock)
	writeLastBlock(config.LastBlockPath, lastBlock)

	whale_txn := parseWhaleTransactions(blocks, config.WhalesAddr, config.MinETHValue)
	fmt.Println(whale_txn)
	appendCSV(config.CsvPath, whale_txn)
}

func test_gweiToETH() {
	num3 := new(big.Int)
	num3.SetString("1334365091086998352", 10)
	fmt.Println("ETH", gweiToETH(*num3))
	num4 := new(big.Int)
	num4.SetString("133436509108699", 10)
	fmt.Println("ETH", gweiToETH(*num4))
}

// записать последний обработанный номер блока
func writeLastBlock(filename string, block uint64) bool {
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		log.Fatalf("failed opening file: %s", err)
	}
	defer file.Close() // Ensure the file is closed
	content := fmt.Sprintf("%d", block)
	if _, err := file.WriteString(content); err != nil {
		log.Fatalf("failed writing to file: %s", err)
	}
	return true
}

// считать последний обработанный номер блока
func readLastBlock(filename string) uint64 {
	file, err := os.Open(filename)
	if err != nil {
		return 0
		log.Fatalf("Error opening file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var numbers []uint64

	for scanner.Scan() {
		line := scanner.Text()
		num, err := strconv.Atoi(line)
		if err != nil {
			log.Printf("Warning: Could not convert line '%s' to int: %v", line, err)
			continue // Skip this line if it's not a valid integer
		}
		numbers = append(numbers, uint64(num))
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("Error during scanning: %v", err)
	}

	return numbers[0]
}

// добавить строки в CSV файл
func appendCSV(filename string, csv string) bool {
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		log.Fatalf("failed opening file: %s", err)
	}
	defer file.Close() // Ensure the file is closed

	// Write the content to the file
	if _, err := file.WriteString(csv); err != nil {
		log.Fatalf("failed writing to file: %s", err)
	}
	return true
}

// вывести число ЕТН с 5 знаками, из gwei / 10 ** 18
func gweiToETH(gwei big.Int) string {
	str := gwei.String()
	val, err := decimal.NewFromString(str)
	if err != nil {
		fmt.Println("ERROR ", err)
		return "0"
	}
	val = val.Shift(-18)
	val = val.Round(5)
	res := fmt.Sprintf("%s", val)
	return res
}

// отфильтровать транзакции на/с бирж, на крупные суммы ЕТН, сформировать CSV
func parseWhaleTransactions(blocks []*types.ParsedBlock, whalesAddrs map[string]string, minETH uint64) string {
	// blocks [] -> "number" -> "transactions"
	// "value": 1334 36509 10869 98352 gwei / 10 ** 18 = 1.334
	fmt.Println("Started parsing WHALE from/to transactions")

	res := ""
	for _, blk := range blocks {
		for _, txn := range blk.Transactions {
			from_name, is_from := whalesAddrs[strings.ToLower(txn.From)]
			tx_value := gweiToETH(*txn.Value)
			sum_tx, err := strconv.ParseFloat(tx_value, 64)
			// пропускаем транзакции c value < minETH
			if err != nil || sum_tx < float64(minETH) {
				continue
			}
			now := time.Now()
			formattedTime := now.Format("2006-01-02 15:04:05")

			if is_from {
				res += fmt.Sprintf("\"https://etherscan.io/tx/%s\",\"%s ETH\",\"FROM\",\"%s\",\"%s\",\"%s\",\"%d\"\n",
					txn.Hash, tx_value, txn.From, from_name, formattedTime, txn.BlockNumber)
			}
			// txn.To == nil - при транзакции с созданием контракта, проверка
			if txn.To != nil {
				to_name, is_to := whalesAddrs[strings.ToLower(*txn.To)]
				if is_to {
					res += fmt.Sprintf("\"https://etherscan.io/tx/%s\",\"%s ETH\",\"TO\",\"%s\",\"%s\",\"%s\",\"%d\"\n",
						txn.Hash, tx_value, *txn.To, to_name, formattedTime, txn.BlockNumber)
				}
			}
		}
	}

	return res
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
