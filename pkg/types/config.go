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
	DumpJsonFile bool   `json:"dump_json_file" yaml:"dump_json_file"`

	// Database settings (if using database output)
	DatabaseURL string `json:"database_url" yaml:"database_url"`

	// Filtering options
	MinETHValue     uint64            `json: "min_eth_value" yaml:"min_eth_value"`
	WhalesAddr      map[string]string `json: "address_names" yaml:"address_names"`
	FilterAddresses []string          `json:"filter_addresses" yaml:"filter_addresses"`
	FilterTopics    []string          `json:"filter_topics" yaml:"filter_topics"`
	IncludeLogs     bool              `json:"include_logs" yaml:"include_logs"`
	IncludeTraces   bool              `json:"include_traces" yaml:"include_traces"`
	CsvPath         string            `json:"csv_path" yaml:"csv_path"`
	LastBlockPath   string            `json:"last_block_path" yaml:"last_block_path"`
	MaxBlockDelta   uint64            `json:"max_block_delta" yaml:"max_block_delta"`

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
		BatchSize:                  10, // Smaller batches for Infura
		Workers:                    5,  // Infura rate limits
		RequestTimeout:             30 * time.Second,
		OutputFormat:               "json",
		OutputPath:                 "./output",
		IncludeLogs:                false, // TODO: true для парсинга токен-транзакций
		IncludeTraces:              false,
		MaxTransactionsForReceipts: 1,    // Skip receipts for blocks with more than N transactions
		SkipReceiptsOnLargeBlocks:  true, // Enable skipping receipts for large blocks
		MinETHValue:                1,    // signal on TXNs with ETH value >= MinETHValue
		WhalesAddr:                 WhaleAddresses(),
		CsvPath:                    "./whale_txns.csv",
		LastBlockPath:              "./last_block.dat",
		MaxBlockDelta:              50,
		DumpJsonFile:               false,
	}
}

// list of top ETH holders with names (exchange wallets)
func WhaleAddresses() map[string]string {
	whales := map[string]string{
		"0x00000000219ab540356cbb839cbe05303d7705fa": "Beacon Deposit Contract",
		"0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2": "Wrapped Ether",
		"0xbe0eb53f46cd790cd13851d5eff43d12404d33e8": "Binance 7",
		"0x40b38765696e3d5d8d9d834d8aad4bb6e418e489": "Robinhood",
		"0x49048044d57e1c92a77f79988d21fa8faf74e97e": "Base: Base Portal",
		"0x0e58e8993100f1cbe45376c410f97f4893d9bfcd": "Upbit 41",
		"0x8315177ab297ba92a06054ce80a67ed4dbd7ed3a": "Arbitrum: Bridge",
		"0xf977814e90da44bfa03b6295a0616a897441acec": "Binance: Hot Wallet 20",
		"0x47ac0fb4f2d84898e4d9e7b4dab3c24507a6d503": "Binance: Binance-Peg Tokens",
		"0xe92d1a43df510f82c66382592a047d288f85226f": "Bitfinex 19",
		"0x61edcdf5bb737adffe5043706e7c5bb1f1a56eea": "Gemini 3",
		"0x3bfc20f0b9afcace800d73d2191166ff16540258": "Polkadot: MultiSig",
		"0xd3a22590f8243f8e83ac230d1842c9af0404c4a1": "Ceffu: Custody Hot Wallet 2",
		"0x8103683202aa8da10536036edef04cdd865c225e": "Bitfinex 20",
		"0x109be9d7d5f64c8c391ced3a8f69bdef20fcaea9": "Kraken 74",
		"0x539c92186f7c6cc4cbf443f26ef84c595babbca1": "OKX 73",
		"0x2b6ed29a95753c3ad948348e3e7b1a251080ffb9": "Rain Lohmus",
		"0xbfbbfaccd1126a11b8f84c60b09859f80f3bd10f": "OKX 93",
		"0x868dab0b8e21ec0a48b726a1ccf25826c78c6d7f": "OKX 76",
		"0x220866b1a2219f40e72f5c628b65d54268ca3a9d": "Vb 3",
		"0xc61b9bb3a7a0767e3179713f3a5c7a9aedce193c": "Bitfinex: MultiSig 2",
		"0x5a52e96bacdabb82fd05763e25335261b270efcb": "Binance 28",
		"0x267be1c1d684f78cb4f6a176c4911b741e4ffdc0": "Kraken 4",
		"0xde0b295669a9fd93d5f28d9ec85e40f4cb697bae": "EthDev",
		"0xd19d4b5d358258f05d7b411e21a1460d11b0876f": "Linea: L1 Message Service",
		"0x5b5b69f4e0add2df5d2176d7dbd20b4897bc7ec4": "QuadrigaCX 4",
		"0x742d35cc6634c0532925a3b844bc454e4438f44e": "Bitfinex 2",
		"0x73af3bcf944a6559933396c1577b257e2054d935": "Robinhood 6",
		"0xa023f08c70a23abc7edfc5b6b5e171d78dfc947e": "Crypto.com 22",
		"0xa160cdab225685da1d56aa342ad8841c3b53f291": "Tornado.Cash: 100 ETH",
		"0x8484ef722627bf18ca5ae6bcf031c23e6e922b30": "Polygon (Matic): Ether Bridge",
		"0x376c3e5547c68bc26240d8dcc6729fff665a4448": "Iconomi: MultiSig 1",
		"0x5a710a3cdf2af218740384c52a10852d8870626a": "Bitfinex 15",
		"0x4fdd5eb2fb260149a3903859043e962ab89d8ed4": "Bitfinex 5",
		"0x28140cb1ac771d4add91ee23788e50249c10263d": "Bitfinex 16",
		"0xc56fefd1028b0534bfadcdb580d3519b5586246e": "Bitfinex 11",
		"0x3727cfcbd85390bb11b3ff421878123adb866be8": "Bitbank 2",
		"0xc882b111a75c0c657fc507c04fbfcd2cc984f071": "Gate.io 5",
		"0xdf9eb223bafbe5c5271415c75aecd68c21fe3d7f": "Liquity: Active Pool",
		"0xb3764761e297d6f121e79c32a65829cd1ddb4d32": "Multisig Exploit Hacker",
		"0xbf4ed7b27f1d666546e30d74d50d173d20bca754": "WithdrawDAO",
		"0xc54cb22944f2be476e02decfcd7e3e7d3e15a8fb": "Mantle: Proxy",
		"0x59708733fbbf64378d9293ec56b977c011a08fd2": "Bitget 23",
		"0x755cdba6ae4f479f7164792b318b2a06c759833b": "ExtraBalDaoWithdraw",
		"0x1e143b2588705dfea63a17f2032ca123df995ce0": "QuadrigaCX 5",
		"0x98adef6f2ac8572ec48965509d69a8dd5e8bba9d": "Binance 93",
		"0xb0a27099582833c0cb8c7a0565759ff145113d64": "OKX 153",
		"0x889edc2edab5f40e902b864ad4d7ade8e412f9b1": "Lido: Withdrawal Queue",
		"0xd69b0089d9ca950640f5dc9931a41a5965f00303": "Gemini 7",
		"0x3bc643a841915a267ee067b580bd802a66001c1d": "BTC Markets 1",
		"0x52e86988bd07447c596e9b0c7765f8500113104c": "Mixin Network Exploiter 1",
		"0x1342a001544b8b7ae4a5d374e33114c66d78bd5f": "Gatecoin Hacker 2",
		"0xd4914762f9bd566bd0882b71af5439c0476d2ff6": "Gatecoin Hacker 1",
		"0xb10edd6fa6067dba8d4326f1c8f0d1c791594f13": "Bitpanda 5",
		"0x1e2fcfd26d36183f1a5d90f0e6296915b02bcb40": "Coinone 2",
		"0xd6216fc19db775df9774a6e33526131da7d19a2c": "KuCoin 6",
		"0xad10a0ec7a7fdd54b9d13fa8e2ee1d5f4e94627a": "VanEck: ETHV Ethereum ETF",
		"0x56eddb7aa87536c09ccc2793473599fd21a8b17f": "Binance 17",
		"0x6e29f75b0350fd0e85ee34a21ef94767b0186996": "Stake.com 3",
	}
	return whales
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
