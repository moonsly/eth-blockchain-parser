package filtering

import (
	"bufio"
	"eth-blockchain-parser/pkg/database"
	"eth-blockchain-parser/pkg/types"
	"fmt"
	"log"
	"math/big"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

func test_gweiToETH() {
	num3 := new(big.Int)
	num3.SetString("1334365091086998352", 10)
	fmt.Println("ETH", gweiToETH(*num3))
	num4 := new(big.Int)
	num4.SetString("133436509108699", 10)
	fmt.Println("ETH", gweiToETH(*num4))
}

// записать последний обработанный номер блока
func WriteLastBlock(filename string, block uint64) bool {
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
func ReadLastBlock(filename string) uint64 {
	file, err := os.Open(filename)
	if err != nil {
		return 0
		// log.Fatalf("Error opening file: %v", err)
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

	if len(numbers) == 0 {
		return 0
	}

	return numbers[0]
}

// добавить строки в CSV файл
func AppendCSV(filename string, csv string) bool {
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

func ParseWhaleTransactions(blocks []*types.ParsedBlock, whalesAddrsID map[string]string,
	minETH uint64) []*database.Transaction {

	fmt.Println("Started parsing WHALE from/to transactions to []")
	// value 1.12345, from/to, whale_id
	res := make([]*database.Transaction, 0)
	for _, blk := range blocks {
		for _, txn := range blk.Transactions {
			whale_id, is_from := whalesAddrsID[strings.ToLower(txn.From)]
			tx_value := gweiToETH(*txn.Value)
			tx_dest := ""
			sum_tx, err := strconv.ParseFloat(tx_value, 64)
			// пропускаем транзакции c value < minETH
			if err != nil || sum_tx < float64(minETH) {
				continue
			}
			now := time.Now()
			formattedTime := now.Format("2006-01-02 15:04:05")

			if is_from {
				tx_dest = "FROM"
			}
			// txn.To == nil - при транзакции с созданием контракта, проверка
			if txn.To != nil {
				whale_to_id, is_to := whalesAddrsID[strings.ToLower(*txn.To)]
				if is_to {
					whale_id = whale_to_id
					tx_dest = "TO"
					if is_from && is_to {
						tx_dest = "INT"
					}
				}
			}
			if tx_dest != "" {
				// map to db.Transaction
				tx_params := []string{tx_value, tx_dest, whale_id}
				db_tx, err := database.MapParsedTxToDatabaseTx(txn, tx_params...)
				if err != nil {
					fmt.Println("ERROR mapping tx", txn.Hash)
				}
				fmt.Println(tx_dest, formattedTime, db_tx, err)
				res = append(res, db_tx)
			}
		}
	}

	return res
}

// перевод txs в формат CSV - используем результат ParseWhaleTransactions
func TransformTxsToCsv(txs []*database.Transaction, whalesAddrs map[string]string) string {
	res := ""
	for _, tx := range txs {
		from_name, is_from := whalesAddrs[strings.ToLower(tx.FromAddress)]
		now := time.Now()
		formattedTime := now.Format("2006-01-02 15:04:05")
		if is_from {
			res += fmt.Sprintf("\"https://etherscan.io/tx/%s\",\"%s ETH\",\"FROM\",\"%s\",\"%s\",\"%s\",\"%d\"\n",
				tx.TxHash, tx.Value, tx.FromAddress, from_name, formattedTime, tx.BlockNumber)
		}
		if tx.ToAddress != nil {
			to_name, is_to := whalesAddrs[strings.ToLower(*tx.ToAddress)]
			if is_to {
				res += fmt.Sprintf("\"https://etherscan.io/tx/%s\",\"%s ETH\",\"TO\",\"%s\",\"%s\",\"%s\",\"%d\"\n",
					tx.TxHash, tx.Value, *tx.ToAddress, to_name, formattedTime, tx.BlockNumber)
			}
		}
	}
	return res
}
