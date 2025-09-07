package filtering

import (
	"eth-blockchain-parser/pkg/types"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestGweiToETH tests the gweiToETH conversion function
func TestGweiToETH(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "1 ETH in gwei",
			input:    "1000000000000000000", // 1 ETH
			expected: "1",
		},
		{
			name:     "0.5 ETH in gwei",
			input:    "500000000000000000", // 0.5 ETH
			expected: "0.5",
		},
		{
			name:     "Small amount",
			input:    "1334365091086998352", // ~1.334 ETH
			expected: "1.33437",
		},
		{
			name:     "Very small amount",
			input:    "133436509108699", // ~0.000133 ETH
			expected: "0.00013",
		},
		{
			name:     "Zero",
			input:    "0",
			expected: "0",
		},
		{
			name:     "Large amount",
			input:    "10000000000000000000000", // 10,000 ETH
			expected: "10000",
		},
		{
			name:     "Precision test",
			input:    "123456789012345678", // ~0.123456789 ETH
			expected: "0.12346",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gwei := new(big.Int)
			gwei.SetString(tt.input, 10)
			result := gweiToETH(*gwei)
			
			// Parse both values as float for comparison due to potential formatting differences
			expected, err1 := strconv.ParseFloat(tt.expected, 64)
			actual, err2 := strconv.ParseFloat(result, 64)
			
			if err1 != nil || err2 != nil {
				t.Fatalf("Failed to parse floats: expected=%s, actual=%s, err1=%v, err2=%v", 
					tt.expected, result, err1, err2)
			}
			
			// Allow small precision difference (0.00001)
			if abs(expected-actual) > 0.00001 {
				t.Errorf("Expected %s, got %s (difference: %f)", tt.expected, result, abs(expected-actual))
			}
		})
	}
}

// TestWriteLastBlock tests writing block numbers to file
func TestWriteLastBlock(t *testing.T) {
	// Create temporary directory for test files
	tempDir, err := os.MkdirTemp("", "filtering_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name      string
		filename  string
		block     uint64
		expectOk  bool
	}{
		{
			name:     "Write normal block number",
			filename: filepath.Join(tempDir, "test1.txt"),
			block:    18500000,
			expectOk: true,
		},
		{
			name:     "Write zero block",
			filename: filepath.Join(tempDir, "test2.txt"),
			block:    0,
			expectOk: true,
		},
		{
			name:     "Write large block number",
			filename: filepath.Join(tempDir, "test3.txt"),
			block:    99999999999,
			expectOk: true,
		},
		{
			name:     "Overwrite existing file",
			filename: filepath.Join(tempDir, "test1.txt"),
			block:    18500001,
			expectOk: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := WriteLastBlock(tt.filename, tt.block)
			
			if result != tt.expectOk {
				t.Errorf("Expected %v, got %v", tt.expectOk, result)
			}

			if tt.expectOk {
				// Verify file content
				content, err := os.ReadFile(tt.filename)
				if err != nil {
					t.Fatalf("Failed to read file: %v", err)
				}
				
				expectedContent := strconv.FormatUint(tt.block, 10)
				if string(content) != expectedContent {
					t.Errorf("Expected file content %s, got %s", expectedContent, string(content))
				}
			}
		})
	}
}

// TestReadLastBlock tests reading block numbers from file
func TestReadLastBlock(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "filtering_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name         string
		filename     string
		fileContent  string
		createFile   bool
		expected     uint64
	}{
		{
			name:        "Read normal block number",
			filename:    filepath.Join(tempDir, "read1.txt"),
			fileContent: "18500000",
			createFile:  true,
			expected:    18500000,
		},
		{
			name:        "Read zero",
			filename:    filepath.Join(tempDir, "read2.txt"),
			fileContent: "0",
			createFile:  true,
			expected:    0,
		},
		{
			name:        "Read multiline (should return first)",
			filename:    filepath.Join(tempDir, "read3.txt"),
			fileContent: "123\n456\n789",
			createFile:  true,
			expected:    123,
		},
		{
			name:       "File doesn't exist",
			filename:   filepath.Join(tempDir, "nonexistent.txt"),
			createFile: false,
			expected:   0,
		},
		{
			name:        "Invalid content",
			filename:    filepath.Join(tempDir, "read4.txt"),
			fileContent: "invalid",
			createFile:  true,
			expected:    0, // Should return 0 since numbers slice will be empty
		},
		{
			name:        "Empty file",
			filename:    filepath.Join(tempDir, "read5.txt"),
			fileContent: "",
			createFile:  true,
			expected:    0, // Should return 0 since numbers slice will be empty
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.createFile {
				err := os.WriteFile(tt.filename, []byte(tt.fileContent), 0644)
				if err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
			}

			result := ReadLastBlock(tt.filename)
			
			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}

// TestAppendCSV tests appending CSV content to files
func TestAppendCSV(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "filtering_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	filename := filepath.Join(tempDir, "test.csv")
	
	tests := []struct {
		name    string
		csv     string
		expectOk bool
	}{
		{
			name:     "Append first line",
			csv:      "column1,column2,column3\n",
			expectOk: true,
		},
		{
			name:     "Append data line",
			csv:      "value1,value2,value3\n",
			expectOk: true,
		},
		{
			name:     "Append empty string",
			csv:      "",
			expectOk: true,
		},
		{
			name:     "Append special characters",
			csv:      "\"quoted,value\",123,\"line\nbreak\"\n",
			expectOk: true,
		},
	}

	expectedContent := ""
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AppendCSV(filename, tt.csv)
			
			if result != tt.expectOk {
				t.Errorf("Expected %v, got %v", tt.expectOk, result)
			}

			expectedContent += tt.csv
			
			// Verify file content
			content, err := os.ReadFile(filename)
			if err != nil {
				t.Fatalf("Failed to read file: %v", err)
			}
			
			if string(content) != expectedContent {
				t.Errorf("Expected file content:\n%s\nGot:\n%s", expectedContent, string(content))
			}
		})
	}
}

// TestParseWhaleTransactions tests the whale transaction parsing functionality
func TestParseWhaleTransactions(t *testing.T) {
	// Create test data
	testBlocks := createTestBlocks()
	whaleAddresses := map[string]string{
		"0x1234567890abcdef1234567890abcdef12345678": "Binance",
		"0xabcdefabcdefabcdefabcdefabcdefabcdefabcd": "Coinbase",
		"0x9876543210fedcba9876543210fedcba98765432": "Kraken",
	}

	tests := []struct {
		name           string
		blocks         []*types.ParsedBlock
		whaleAddrs     map[string]string
		minETH         uint64
		expectedLines  int
		shouldContain  []string
		shouldNotContain []string
	}{
		{
			name:          "Filter with minimum 1 ETH",
			blocks:        testBlocks,
			whaleAddrs:    whaleAddresses,
			minETH:        1,
			expectedLines: 4, // 2 from whale FROM + 2 to whale TO
			shouldContain: []string{
				"\"FROM\"",
				"\"TO\"",
				"\"Binance\"",
				"\"Coinbase\"",
				"\"2 ETH\"",
				"\"5 ETH\"",
				"etherscan.io/tx/",
			},
			shouldNotContain: []string{
				"\"0.5 ETH\"", // Below minimum
			},
		},
		{
			name:          "Filter with minimum 3 ETH",
			blocks:        testBlocks,
			whaleAddrs:    whaleAddresses,
			minETH:        3,
			expectedLines: 2, // Only 5 ETH transactions
			shouldContain: []string{
				"\"5 ETH\"",
			},
			shouldNotContain: []string{
				"\"2 ETH\"",
				"\"0.5 ETH\"",
			},
		},
		{
			name:          "No matching addresses",
			blocks:        testBlocks,
			whaleAddrs:    map[string]string{
				"0xnonexistent1": "Exchange1",
				"0xnonexistent2": "Exchange2",
			},
			minETH:        1,
			expectedLines: 0,
			shouldContain: []string{},
			shouldNotContain: []string{
				"\"FROM\"",
				"\"TO\"",
			},
		},
		{
			name:          "Empty blocks",
			blocks:        []*types.ParsedBlock{},
			whaleAddrs:    whaleAddresses,
			minETH:        1,
			expectedLines: 0,
			shouldContain: []string{},
			shouldNotContain: []string{},
		},
		{
			name:          "Very high minimum",
			blocks:        testBlocks,
			whaleAddrs:    whaleAddresses,
			minETH:        100,
			expectedLines: 0,
			shouldContain: []string{},
			shouldNotContain: []string{
				"\"FROM\"",
				"\"TO\"",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseWhaleTransactions(tt.blocks, tt.whaleAddrs, tt.minETH)
			
			// Count lines
			lines := strings.Split(strings.TrimSpace(result), "\n")
			actualLines := 0
			if result != "" {
				actualLines = len(lines)
			}
			
			if actualLines != tt.expectedLines {
				t.Errorf("Expected %d lines, got %d. Result:\n%s", tt.expectedLines, actualLines, result)
			}

			// Check for expected content
			for _, expected := range tt.shouldContain {
				if !strings.Contains(result, expected) {
					t.Errorf("Result should contain %s, but doesn't. Result:\n%s", expected, result)
				}
			}

			// Check for unexpected content
			for _, unexpected := range tt.shouldNotContain {
				if strings.Contains(result, unexpected) {
					t.Errorf("Result should not contain %s, but does. Result:\n%s", unexpected, result)
				}
			}

			// Validate CSV format if result is not empty
			if result != "" {
				validateCSVFormat(t, result)
			}
		})
	}
}

// TestParseWhaleTransactionsEdgeCases tests edge cases for whale transaction parsing
func TestParseWhaleTransactionsEdgeCases(t *testing.T) {
	// Test with nil To address (contract creation)
	nilToBlock := &types.ParsedBlock{
		Number: 18500000,
		Transactions: []*types.ParsedTransaction{
			{
				Hash:        "0xcontract123",
				BlockNumber: 18500000,
				From:        "0x1234567890abcdef1234567890abcdef12345678", // Whale address
				To:          nil, // Contract creation
				Value:       big.NewInt(2000000000000000000), // 2 ETH
			},
		},
	}

	whaleAddrs := map[string]string{
		"0x1234567890abcdef1234567890abcdef12345678": "Binance",
	}

	result := ParseWhaleTransactions([]*types.ParsedBlock{nilToBlock}, whaleAddrs, 1)
	
	// Should have FROM entry but no TO entry (since To is nil)
	lines := strings.Split(strings.TrimSpace(result), "\n")
	if len(lines) != 1 {
		t.Errorf("Expected 1 line for contract creation, got %d", len(lines))
	}
	
	if !strings.Contains(result, "\"FROM\"") {
		t.Error("Should contain FROM entry for contract creation")
	}
	
	if strings.Contains(result, "\"TO\"") {
		t.Error("Should not contain TO entry for contract creation (To is nil)")
	}
}

// Helper function to create test blocks
func createTestBlocks() []*types.ParsedBlock {
	return []*types.ParsedBlock{
		{
			Number: 18500000,
			Transactions: []*types.ParsedTransaction{
				{
					Hash:        "0xhash1",
					BlockNumber: 18500000,
					From:        "0x1234567890abcdef1234567890abcdef12345678", // Whale address (Binance)
					To:          stringPtr("0xregularuser1"),
					Value:       big.NewInt(2000000000000000000), // 2 ETH
				},
				{
					Hash:        "0xhash2",
					BlockNumber: 18500000,
					From:        "0xregularuser2",
					To:          stringPtr("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd"), // Whale address (Coinbase)
					Value:       big.NewInt(5000000000000000000), // 5 ETH
				},
				{
					Hash:        "0xhash3",
					BlockNumber: 18500000,
					From:        "0xregularuser3",
					To:          stringPtr("0xregularuser4"),
					Value:       big.NewInt(500000000000000000), // 0.5 ETH (below minimum)
				},
			},
		},
		{
			Number: 18500001,
			Transactions: []*types.ParsedTransaction{
				{
					Hash:        "0xhash4",
					BlockNumber: 18500001,
					From:        "0x9876543210fedcba9876543210fedcba98765432", // Whale address (Kraken)
					To:          stringPtr("0xregularuser5"),
					Value:       big.NewInt(5000000000000000000), // 5 ETH
				},
				{
					Hash:        "0xhash5",
					BlockNumber: 18500001,
					From:        "0xregularuser6",
					To:          stringPtr("0x1234567890abcdef1234567890abcdef12345678"), // Whale address (Binance)
					Value:       big.NewInt(2000000000000000000), // 2 ETH
				},
				{
					Hash:        "0xhash6",
					BlockNumber: 18500001,
					From:        "0xregularuser7",
					To:          stringPtr("0xnonwhale"),
					Value:       func() *big.Int { val := new(big.Int); val.SetString("10000000000000000000", 10); return val }(), // 10 ETH but no whale involved
				},
			},
		},
	}
}

// Helper function to validate CSV format
func validateCSVFormat(t *testing.T, csvContent string) {
	lines := strings.Split(strings.TrimSpace(csvContent), "\n")
	
	for i, line := range lines {
		if line == "" {
			continue
		}
		
		// Each line should have exactly 7 comma-separated values (quoted)
		// Format: "URL","VALUE","TYPE","ADDRESS","NAME","TIMESTAMP","BLOCK_NUMBER"
		parts := strings.Split(line, "\",\"")
		if len(parts) != 7 {
			t.Errorf("Line %d has %d parts, expected 7: %s", i+1, len(parts), line)
		}
		
		// First part should start with quote
		if !strings.HasPrefix(parts[0], "\"") {
			t.Errorf("Line %d should start with quote: %s", i+1, line)
		}
		
		// Last part should end with quote
		if !strings.HasSuffix(parts[6], "\"") {
			t.Errorf("Line %d should end with quote: %s", i+1, line)
		}
		
		// URL should contain etherscan
		if !strings.Contains(parts[0], "etherscan.io") {
			t.Errorf("Line %d should contain etherscan URL: %s", i+1, line)
		}
		
		// Type should be FROM or TO
		typeField := strings.Trim(parts[2], "\"")
		if typeField != "FROM" && typeField != "TO" {
			t.Errorf("Line %d should have type FROM or TO, got %s: %s", i+1, typeField, line)
		}
		
		// Value should contain ETH
		if !strings.Contains(parts[1], "ETH") {
			t.Errorf("Line %d should contain ETH in value field: %s", i+1, line)
		}
	}
}

// Helper function to create string pointer
func stringPtr(s string) *string {
	return &s
}

// Helper function to get absolute difference between floats
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// TestIntegrationFullWorkflow tests the complete workflow of the filtering module
func TestIntegrationFullWorkflow(t *testing.T) {
	// Create temporary directory for integration test
	tempDir, err := os.MkdirTemp("", "filtering_integration_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test files
	blockFile := filepath.Join(tempDir, "last_block.txt")
	csvFile := filepath.Join(tempDir, "whale_transactions.csv")

	// Step 1: Write initial block number
	initialBlock := uint64(18500000)
	if !WriteLastBlock(blockFile, initialBlock) {
		t.Fatal("Failed to write initial block")
	}

	// Step 2: Read block number back
	readBlock := ReadLastBlock(blockFile)
	if readBlock != initialBlock {
		t.Fatalf("Expected block %d, got %d", initialBlock, readBlock)
	}

	// Step 3: Process whale transactions
	testBlocks := createTestBlocks()
	whaleAddrs := map[string]string{
		"0x1234567890abcdef1234567890abcdef12345678": "Binance",
		"0xabcdefabcdefabcdefabcdefabcdefabcdefabcd": "Coinbase",
	}

	csvContent := ParseWhaleTransactions(testBlocks, whaleAddrs, 1)
	
	// Step 4: Append CSV content
	if !AppendCSV(csvFile, csvContent) {
		t.Fatal("Failed to append CSV content")
	}

	// Step 5: Verify CSV file content
	savedContent, err := os.ReadFile(csvFile)
	if err != nil {
		t.Fatalf("Failed to read CSV file: %v", err)
	}

	if string(savedContent) != csvContent {
		t.Error("CSV file content doesn't match expected content")
	}

	// Step 6: Update block number
	newBlock := uint64(18500002)
	if !WriteLastBlock(blockFile, newBlock) {
		t.Fatal("Failed to update block number")
	}

	// Step 7: Verify updated block number
	updatedBlock := ReadLastBlock(blockFile)
	if updatedBlock != newBlock {
		t.Fatalf("Expected updated block %d, got %d", newBlock, updatedBlock)
	}

	t.Log("Integration test completed successfully")
}

// BenchmarkGweiToETH benchmarks the gweiToETH function
func BenchmarkGweiToETH(b *testing.B) {
	gwei := new(big.Int)
	gwei.SetString("1334365091086998352", 10)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gweiToETH(*gwei)
	}
}

// BenchmarkParseWhaleTransactions benchmarks the whale transaction parsing
func BenchmarkParseWhaleTransactions(b *testing.B) {
	testBlocks := createTestBlocks()
	whaleAddrs := map[string]string{
		"0x1234567890abcdef1234567890abcdef12345678": "Binance",
		"0xabcdefabcdefabcdefabcdefabcdefabcdefabcd": "Coinbase",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ParseWhaleTransactions(testBlocks, whaleAddrs, 1)
	}
}
