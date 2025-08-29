# Ethereum Blockchain Parser with Infura Support

A Go-based Ethereum blockchain parser that works with both local Geth nodes and Infura API.

## Quick Start with Infura (Recommended)

### 1. Set your Infura API Key

```bash
export INFURA_API_KEY="your-api-key-here"
```

### 2. Run the Parser

```bash
# Build the simple CLI
go build -o infura-parser ./cmd/infura-parser

# Parse last 10 blocks from mainnet
./infura-parser

# Parse specific block range
./infura-parser -start=18000000 -end=18000010

# Parse from a different network
./infura-parser -network=sepolia

# Filter by specific address
./infura-parser -filter-address="0x742d35Cc6534C0532925a3b8D"

# Save to file
./infura-parser -output=blocks.json
```

## Configuration Options

### Using Environment Variables

```bash
export INFURA_API_KEY="your-api-key-here"
export INFURA_NETWORK="mainnet"  # optional, defaults to mainnet
```

### Using Command Line Flags

```bash
./infura-parser \
  -infura-api-key="your-key-here" \
  -network="mainnet" \
  -start="latest-100" \
  -end="latest" \
  -workers=2 \
  -batch-size=5 \
  -logs=true \
  -output="output.json"
```

### Using Configuration File

Create a `config.yaml`:

```yaml
use_infura: true
infura_api_key: "your-api-key-here"  # Your Infura Project ID
infura_network: "mainnet"
network_id: 1
start_block: 18000000
end_block: 18000010
workers: 2
batch_size: 5
include_logs: true
output_format: "json"
```

## Supported Networks

- `mainnet` - Ethereum Mainnet
- `goerli` - Goerli Testnet
- `sepolia` - Sepolia Testnet
- `polygon-mainnet` - Polygon Mainnet
- `polygon-mumbai` - Polygon Mumbai Testnet
- `arbitrum-mainnet` - Arbitrum One
- `arbitrum-goerli` - Arbitrum Goerli
- `optimism-mainnet` - Optimism Mainnet
- `optimism-goerli` - Optimism Goerli

## Example Usage

### Parse Recent Blocks

```go
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
    // Get your Infura API key
    apiKey := os.Getenv("INFURA_API_KEY")
    
    // Create client
    ethClient, err := client.NewInfuraClientSimple(apiKey, "mainnet")
    if err != nil {
        log.Fatal(err)
    }
    defer ethClient.Close()
    
    // Create parser
    config := types.InfuraConfigSimple(apiKey, "mainnet")
    blockParser := parser.NewParser(ethClient, config)
    
    // Parse latest block
    latest, _ := ethClient.GetLatestBlockNumber(context.Background())
    block, err := blockParser.ParseSingleBlock(context.Background(), latest)
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Block %d has %d transactions\n", block.Number, len(block.Transactions))
}
```

## Features

- **Infura API Support** - Works with just your Infura API key
- **Rate Limiting** - Built-in rate limiting for Infura free tier
- **Batch Processing** - Efficient batch requests for better performance
- **Multi-network** - Supports all major Ethereum networks
- **Transaction Parsing** - Detailed transaction information including logs
- **Token Transfer Detection** - Identifies ERC20/721/1155 transfers
- **Contract Interaction Analysis** - Analyzes smart contract calls
- **Concurrent Processing** - Multi-worker architecture for speed
- **Error Recovery** - Automatic retry with exponential backoff

## Rate Limits

The parser automatically handles Infura rate limits:

- **Free Tier**: 3 mln points / day
- **Paid Tiers**: Higher limits based on your plan

The parser uses conservative settings:
- 500ms delay between requests on rate limit error
- Automatic retry on rate limit errors

## Output Formats

### JSON (Default)
```json
{
  "number": 18000000,
  "hash": "0x...",
  "transactions": [...],
  "gas_used": 15000000
}
```

### With Transaction Details
```json
{
  "transactions": [{
    "hash": "0x...",
    "from": "0x...",
    "to": "0x...",
    "value": "1000000000000000000",
    "gas_used": 21000,
    "logs": [...]
  }]
}
```

## Error Handling

The parser handles common issues automatically:

- **Network disconnections** - Automatic reconnection
- **Rate limiting** - Built-in delays and retries
- **Invalid blocks** - Graceful error handling
- **Partial failures** - Continues processing other blocks

## Building

```bash
# Install dependencies
go mod tidy

# Build the simple CLI
go build -o infura-parser ./cmd/infura-parser

# Build the full CLI
go build -o eth-parser ./cmd

# Run tests
go test ./...
```

## Troubleshooting

### "Invalid Infura configuration"
- Make sure you're using your Project ID (what Infura calls "API Key")
- Check that the network name is supported

### "Rate limit exceeded"
- Reduce workers and batch size
- The parser will automatically retry after delays

### "Connection failed"
- Check your API key is correct
- Verify the network name is valid
- Check your internet connection
