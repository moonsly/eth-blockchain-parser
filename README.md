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

## Дополнительно сделано 
1) сохранение отфильтрованных транзакций в БД Sqlite3 + в CSV
2) лок через syscall.Flock, чтобы работал всегда только один инстанс парсера (например, если по крону предыдущий еще не завершился - нельзя запускать 2й инстанс)
3) схемы таблиц, создание схем, индексы в БД
4) инициализация таблицы whale_addresses в БД значениями из config.WhalesAddr (запуск с параметром -initw)
5) частичное покрытие автотестами - для пакета filtering
6) JSON API на net/http с basic HTTP авторизацией
7) запуск на своем хостинге, тестирование несколько дней с накоплением записей в БД
8) регулярная очистка старых записей в БД (старше 14 дней)
9) Dockerfile

## Запуск в Docker

```bash
cp .env.example .env

# set INFURA_API_KEY in .env
vim .env

docker-compose build

docker-compose up -d

# logs

docker-compose logs -f

```
После запуска - можно заходить в админку с транзакциями: 

http://localhost:8015/api/transactions 

admin/password123

## CURL для тестирования (АПИ + воркер развернуты на хостинге)

JSON API логин/пасс:

http://lnkweb.ru:8015/api

admin/password123

```bash
# все транзакции

curl -u "admin:password123" -H "Content-type: application/json" -s -X GET http://lnkweb.ru:8015/api/transactions | jq

# транзакции с пагинацией

curl -u "admin:password123" -G "http://lnkweb.ru:8015/api/transactions" -d limit=3 -d page=3
{"success":true,"data":[{"id":71,"tx_hash":"0xb8060760673bbc0e4cae6ea1e98a60e10623cb4e65e7990b111289edaa4b6142","block_number":23328197,"block_hash":"0xbced5bfb77c689773c1213c2df635eeab2805f4620f8af25fe53aeeb8702fc9d","transaction_index":150,"from_address":"0x56Eddb7aa87536c09CCc2793473599fD21A8b17F",...}],"count":3,"meta":{"page":3,"limit":3,"total":66,"has_next":true,"has_prev":true}}

# 1 транзакция по tx_hash

curl -u "admin:password123" -H "Content-type: application/json" -s -X GET http://lnkweb.ru:8015/api/transactions/0x3bb4c67c987ae8e2b383370a19ba1f634f5c7535446d5074ddfc42018700b5c0 | jq
{
  "success": true,
  "data": {
    "id": 72,
    "tx_hash": "0x3bb4c67c987ae8e2b383370a19ba1f634f5c7535446d5074ddfc42018700b5c0",
    ...
  }
}

# все транзакции по кошельку 0x56Eddb7aa87536c09CCc2793473599fD21A8b17F

curl -u "admin:password123" -H "Content-type: application/json" -s -X GET http://lnkweb.ru:8015/api/addresses/0x56Eddb7aa87536c09CCc2793473599fD21A8b17F/transactions
```

## Особенности реализации

### 1. Сборка и запуск с Infura API Key (можно получить после бесплатной регистрации)

```bash
export INFURA_API_KEY="your-api-key-here"

go run ./cmd/infura-parser/main.go

# build
cd /home/zak/work/eth-blockchain-parser

rm ./server-run; go build -o server-run ./cmd/server-run/

rm ./infura-parser; go build -o infura-parser ./cmd/infura-parser/
```

### 2. Настройки числа воркеров для управления рейт-лимитами infura

[config.go](pkg/types/config.go)

```bash
    BatchSize:                  10, // Smaller batches for Infura
	Workers:                    5,  // Infura rate limits
	RequestTimeout:             30 * time.Second,
```

### 3. Инициализация whale_addresses БД из конфига config.WhalesAddr

```bash
go run ./cmd/infura-parser/main.go -initw
```

### 4. Добавление в крон задачи

```bash
crontab -e 

*/2 * * * * cd /home/zak/work/eth-blockchain-parser && INFURA_API_KEY="abc_infura_key" ./infura-parser 2>&1 >> /var/log/eth_parser/eth_parser.log
```

### 5. Запуск автотестов (для пакета filtering) 

```bash
go test -v ./internal/filtering/
=== RUN   TestGweiToETH
=== RUN   TestGweiToETH/1_ETH_in_gwei
=== RUN   TestGweiToETH/0.5_ETH_in_gwei
...
# benchmarks
go test ./internal/filtering -bench=.
```

### 6. Просмотр CSV с результатами парсинга, MinETHValue = 1

```bash
 tail ./whale_txns.csv 
...
"https://etherscan.io/tx/0x2b8a54ff684db28cfa1b8d21799b1a727298ce234c63ef49ebb4cee51ca938db","120 ETH","TO","0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2","Wrapped Ether","2025-09-07 22:35:04","23314194"
"https://etherscan.io/tx/0xa0414806bfbd5f1e1b6283c06009937c8a3d042cb4b918243e5e80f3b11f2fb5","430.9999 ETH","FROM","0x267be1C1D684F78cb4F6a176C4911b741E4Ffdc0","Kraken 4","2025-09-07 22:35:04","23314204"
"https://etherscan.io/tx/0xfe862c23d7343eaa7b9e3aabdcdb14afa281e9dcbfa23681ac8d65fa7f02b17a","7.13425 ETH","TO","0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2","Wrapped Ether","2025-09-07 22:35:04","23314213"
"https://etherscan.io/tx/0x34d0b0d89cb868deeead9f04f75598df92a41309cb34d5e777044f01c72bc797","20.13652 ETH","FROM","0x267be1C1D684F78cb4F6a176C4911b741E4Ffdc0","Kraken 4","2025-09-07 22:35:04","23314218"
"https://etherscan.io/tx/0xf51aa420c383060b3d35ef8b4f006336784da4481b906588c36f0ef090d1f926","3.6009 ETH","FROM","0x267be1C1D684F78cb4F6a176C4911b741E4Ffdc0","Kraken 4","2025-09-07 22:35:04","23314217"
```

### 7. Пример БД с результатами парсинга, MinETHValue = 1
```sql
sqlite> select id, tx_hash, whale_address_id wid, value, transfer_type tt, strftime('%d.%m %H:%M:%S', created_at) time from transactions order by id desc limit 20;
id|tx_hash|wid|value|tt|time
270|0xf43527372a7efabf205c34b65a7bcc81f34db278900523899ff37482b6a30f5e|11|2.0127|FROM|08.09 21:55:38
269|0xfcc36a4cba8fd48e863646156903fd88be8e3cefbde578b0a9ae6aefeee121f7|11|1.32015|FROM|08.09 21:55:38
268|0x4c657a9340e3621691a0dfb50499315ce993ebbc44a186bd4e111e06be218631|11|1.17812|FROM|08.09 21:55:38
267|0x41b5e154e7a99627e20c8b7924dc02eac284eb2cccb5ca2f6037ad84d130f689|24|12.607|FROM|08.09 21:55:38
266|0xb2e779de347f754cc11823f5401a1cf2a355f68463ec476a17a3092f4295f221|11|1.53962|FROM|08.09 21:48:24
...
```
