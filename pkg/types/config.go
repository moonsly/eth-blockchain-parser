package types

import (
	"fmt"
	"time"
)

// Config holds the configuration for the Ethereum blockchain parser
type Config struct {
	// Ethereum node connection settings
	NodeURL   string `json:"node_url" yaml:"node_url"`
	WSNodeURL string `json:"ws_node_url" yaml:"ws_node_url"`
	NetworkID uint64 `json:"network_id" yaml:"network_id"`

	// Infura API settings
	InfuraAPIKey    string `json:"infura_api_key" yaml:"infura_api_key"`       // This is actually the Project ID from Infura dashboard
	InfuraAPISecret string `json:"infura_api_secret" yaml:"infura_api_secret"` // Optional secret for paid plans
	UseInfura       bool   `json:"use_infura" yaml:"use_infura"`
	InfuraNetwork   string `json:"infura_network" yaml:"infura_network"` // mainnet, goerli, sepolia, polygon-mainnet, etc.

	// Parser settings
	StartBlock     uint64        `json:"start_block" yaml:"start_block"`
	EndBlock       uint64        `json:"end_block" yaml:"end_block"`
	BatchSize      uint64        `json:"batch_size" yaml:"batch_size"`
	Workers        int           `json:"workers" yaml:"workers"`
	RequestTimeout time.Duration `json:"request_timeout" yaml:"request_timeout"`

	// Output settings
	OutputFormat string `json:"output_format" yaml:"output_format"` // json, csv, database
	OutputPath   string `json:"output_path" yaml:"output_path"`

	// Database settings (if using database output)
	DatabaseURL string `json:"database_url" yaml:"database_url"`

	// Filtering options
	FilterAddresses []string `json:"filter_addresses" yaml:"filter_addresses"`
	FilterTopics    []string `json:"filter_topics" yaml:"filter_topics"`
	IncludeLogs     bool     `json:"include_logs" yaml:"include_logs"`
	IncludeTraces   bool     `json:"include_traces" yaml:"include_traces"`

	// Receipt processing options
	MaxTransactionsForReceipts int  `json:"max_transactions_for_receipts" yaml:"max_transactions_for_receipts"`
	SkipReceiptsOnLargeBlocks  bool `json:"skip_receipts_on_large_blocks" yaml:"skip_receipts_on_large_blocks"`
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		NodeURL:                    "http://localhost:8545",
		WSNodeURL:                  "ws://localhost:8546",
		NetworkID:                  1, // Mainnet
		InfuraNetwork:              "mainnet",
		BatchSize:                  5, // Smaller batches for Infura
		Workers:                    1, // Single worker to avoid rate limits
		RequestTimeout:             30 * time.Second,
		OutputFormat:               "json",
		OutputPath:                 "./output",
		IncludeLogs:                false, // true для парсинга токен-транзакций
		IncludeTraces:              false,
		MaxTransactionsForReceipts: 10,   // Skip receipts for blocks with more than N transactions
		SkipReceiptsOnLargeBlocks:  true, // Enable skipping receipts for large blocks
	}
}

// InfuraConfig creates a configuration for Infura API using Project ID
func InfuraConfig(projectID, apiSecret, network string) *Config {
	config := DefaultConfig()
	config.UseInfura = true
	config.InfuraAPIKey = projectID // The "API Key" field stores the Project ID
	config.InfuraAPISecret = apiSecret
	config.InfuraNetwork = network
	config.NodeURL = config.BuildInfuraHTTPURL()
	config.WSNodeURL = config.BuildInfuraWSURL()

	// Set network ID based on network name
	switch network {
	case "mainnet":
		config.NetworkID = 1
	case "goerli":
		config.NetworkID = 5
	case "sepolia":
		config.NetworkID = 11155111
	case "polygon-mainnet":
		config.NetworkID = 137
	case "polygon-mumbai":
		config.NetworkID = 80001
	case "arbitrum-mainnet":
		config.NetworkID = 42161
	case "arbitrum-goerli":
		config.NetworkID = 421613
	case "optimism-mainnet":
		config.NetworkID = 10
	case "optimism-goerli":
		config.NetworkID = 420
	default:
		config.NetworkID = 1 // Default to mainnet
	}

	return config
}

// InfuraConfigSimple creates a configuration using just the API key (which is the Project ID)
func InfuraConfigSimple(apiKey, network string) *Config {
	return InfuraConfig(apiKey, "", network)
}

// BuildInfuraHTTPURL constructs the Infura HTTP URL
func (c *Config) BuildInfuraHTTPURL() string {
	if !c.UseInfura || c.InfuraAPIKey == "" {
		return c.NodeURL
	}

	// Use InfuraAPIKey as the Project ID
	baseURL := fmt.Sprintf("https://%s.infura.io/v3/%s", c.InfuraNetwork, c.InfuraAPIKey)
	return baseURL
}

// BuildInfuraWSURL constructs the Infura WebSocket URL
func (c *Config) BuildInfuraWSURL() string {
	if !c.UseInfura || c.InfuraAPIKey == "" {
		return c.WSNodeURL
	}

	// Use InfuraAPIKey as the Project ID
	baseURL := fmt.Sprintf("wss://%s.infura.io/ws/v3/%s", c.InfuraNetwork, c.InfuraAPIKey)
	return baseURL
}

// ValidateInfuraConfig validates Infura configuration
func (c *Config) ValidateInfuraConfig() error {
	if !c.UseInfura {
		return nil
	}

	if c.InfuraAPIKey == "" {
		return fmt.Errorf("infura_api_key (Project ID) is required when use_infura is true.\n" +
			"Your Infura 'API Key' is actually your Project ID.\n" +
			"You can find it in your Infura dashboard.")
	}

	if c.InfuraNetwork == "" {
		c.InfuraNetwork = "mainnet" // Default to mainnet
	}

	// Validate network name
	validNetworks := map[string]bool{
		"mainnet": true, "goerli": true, "sepolia": true,
		"polygon-mainnet": true, "polygon-mumbai": true,
		"arbitrum-mainnet": true, "arbitrum-goerli": true,
		"optimism-mainnet": true, "optimism-goerli": true,
	}

	if !validNetworks[c.InfuraNetwork] {
		return fmt.Errorf("unsupported infura network: %s", c.InfuraNetwork)
	}

	return nil
}
