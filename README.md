# Задание

Необходимо реализовать парсер транзакций эфирного блокчейна ЕТН с рейт-лимитами, на базе модуля go-ethereum.
Цель работы парсера - получать сигналы о крупных ETH транзакциях на кошельках бирж, с задержкой не более 1 минуты.

* В качестве ETH API используем бесплатный АПИ infura.io 
* Парсер умеет запускать Н воркеров в параллель, чтобы подстравиваться под лимиты Infura.
* Парсер может фильтровать транзакции по указанному в конфиге списку кошельков config.WhalesAddr И у которых txn.value >= MinETHValue, список адресов бирж WhalesAddr берется из https://etherscan.io/accounts/1?ps=100
* Парсер запускается по крону и не должен пропускать блоки
* Число воркеров, infura API key, WhalesAddr, MinETHValue и другие параметры задаются в конфиге парсера
* Парсер корректно парсит txn.value в gwei (10**18) и преобразует его в число с 5 нулями value=1.12345
* Отфильтрованные транзакции парсер сохраняет в CSV файл

Дополнительные задания:
1) Парсер умеет восстанавливаться после падения (нет сети/ребут сервера/другие проблемы), не пропускает блоки при этом
2) Настроить число воркеров так, чтобы не превышать лимиты бесплатного API infura
3) Новый блок ЕТН майнится каждые 10-12 секунд, настроить конфиги и крон так, чтобы парсер не отставал от latest блока эфира


## Особенности реализации

### 1. Запуск с Infura API Key (можно получить после бесплатной регистрации)

```bash
export INFURA_API_KEY="your-api-key-here"

go run ./cmd/infura-parser/main.go
```

### 2. Настройки числа воркеров для управления рейт-лимитами infura

[config.go](pkg/types/config.go)

```bash
    BatchSize:                  10, // Smaller batches for Infura
	Workers:                    5,  // Infura rate limits
	RequestTimeout:             30 * time.Second,
```

### 3. Добавление в крон задачи

```bash
crontab -e 

* * * * * /home/user/infura-parser >> /var/log/eth_parser.log
```

### 4. Запуск автотестов (для пакета filtering) 

```bash
go test -v ./pkg/filtering/
=== RUN   TestGweiToETH
=== RUN   TestGweiToETH/1_ETH_in_gwei
=== RUN   TestGweiToETH/0.5_ETH_in_gwei
...
# benchmarks
go test -bench ./pkg/filtering/
```

### 5. Просмотр CSV с результатами парсинга, MinETHValue = 1

```bash
 tail ./whale_txns.csv 
...
"https://etherscan.io/tx/0x2b8a54ff684db28cfa1b8d21799b1a727298ce234c63ef49ebb4cee51ca938db","120 ETH","TO","0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2","Wrapped Ether","2025-09-07 22:35:04","23314194"
"https://etherscan.io/tx/0xa0414806bfbd5f1e1b6283c06009937c8a3d042cb4b918243e5e80f3b11f2fb5","430.9999 ETH","FROM","0x267be1C1D684F78cb4F6a176C4911b741E4Ffdc0","Kraken 4","2025-09-07 22:35:04","23314204"
"https://etherscan.io/tx/0xfe862c23d7343eaa7b9e3aabdcdb14afa281e9dcbfa23681ac8d65fa7f02b17a","7.13425 ETH","TO","0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2","Wrapped Ether","2025-09-07 22:35:04","23314213"
"https://etherscan.io/tx/0x34d0b0d89cb868deeead9f04f75598df92a41309cb34d5e777044f01c72bc797","20.13652 ETH","FROM","0x267be1C1D684F78cb4F6a176C4911b741E4Ffdc0","Kraken 4","2025-09-07 22:35:04","23314218"
"https://etherscan.io/tx/0xf51aa420c383060b3d35ef8b4f006336784da4481b906588c36f0ef090d1f926","3.6009 ETH","FROM","0x267be1C1D684F78cb4F6a176C4911b741E4Ffdc0","Kraken 4","2025-09-07 22:35:04","23314217"
```

