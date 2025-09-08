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

### В процессе реализации:

5) JSON API на net/http с basic HTTP авторизацией
6) запуск на своем хостинге, тестирование несколько дней с накоплением записей в БД
7) Dockerfile
8) регулярная очистка старых записей в БД (старше месяца, число дней в конфиге)

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

### 6. Пример БД с результатами парсинга, MinETHValue = 1
```sql
sqlite> select id, tx_hash, whale_address_id wid, value, transfer_type tt, strftime('%d.%m %H:%M:%S', created_at) time from transactions order by id desc limit 20;
id|tx_hash|wid|value|tt|time
270|0xf43527372a7efabf205c34b65a7bcc81f34db278900523899ff37482b6a30f5e|11|2.0127|FROM|08.09 21:55:38
269|0xfcc36a4cba8fd48e863646156903fd88be8e3cefbde578b0a9ae6aefeee121f7|11|1.32015|FROM|08.09 21:55:38
268|0x4c657a9340e3621691a0dfb50499315ce993ebbc44a186bd4e111e06be218631|11|1.17812|FROM|08.09 21:55:38
267|0x41b5e154e7a99627e20c8b7924dc02eac284eb2cccb5ca2f6037ad84d130f689|24|12.607|FROM|08.09 21:55:38
266|0xb2e779de347f754cc11823f5401a1cf2a355f68463ec476a17a3092f4295f221|11|1.53962|FROM|08.09 21:48:24
265|0x14d671fb1876f7f7af159be79253bab1035ca85e3194348e12a6739ac1b024a4|1|1.39995|TO|08.09 21:48:24
264|0x35d1d0f7e8e376e01775da362fcc5d73e98016e076cee6959a12a0e39ff97b25|1|3.11599|TO|08.09 21:48:24
263|0x51ef54dd11b6dd8bf76c86dafc158940e0729d372ce20fa85fe2e7072bff8a0a|11|1.00472|FROM|08.09 21:48:24
262|0xb0ec71b9a9ed6860353b58c74327f09ff3bf039b03b5a99f5229d00a0c5d9662|1|1.44714|TO|08.09 21:48:24
```
