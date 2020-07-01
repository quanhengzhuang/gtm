# GTM
GTM's full name is `Global Transaction Manager`, a framework for solving distributed transaction problems. GTM is improved based on 2PC, and easier to use than 2PC. Compared to 2PC, which requires participants to implement three functions, many participants in GTM only need to implement one function.

## Transaction Partner
`Partner` is the participant of GTM transaction, used to encapsulate the business logic to be executed. Partners are divided into three types in GTM, which can be applied to different business scenarios:
- `NormalPartner` is a participant that need to support rollback. You can implement `Do() + Undo()` to execute business logic and rollback, and DoNext() only return true to ignore, just like a participant in Saga. You can also implement `Do() + DoNext() + Undo()` to lock resources, execute business logic, and unlock resources, just like a participant in 2PC. NormalPartner is executed first in a GTM transaction, and can be any number.
- `UncertainPartner` is a participant who does not need to support rollback, and the results may succeed or fail. You only need to implement a `Do()` method. UncertainPartner is executed after NormalPartners, and at most one is allowed in a GTM transaction.
- `CertainPartner` is a participant who does not need to support rollback and needs to guarantee success in business logic. You only need to implement a `DoNext()` method. CertainPartner is executed after UncertainPartner, there can be any number in a GTM transaction.

About partners need to implement methods are as follows:
| | Do() | DoNext() | Undo() |
| - | - | - | - |
| NormalPartner | Yes | Optional | Yes |
| UncertainPartner | Yes | | |
| CertainPartner | | Yes | |

## Usage
### Install
```
go get github.com/quanhengzhuang/gtm
```

### Set the Storage
`DBStorage` provided by GTM is used here, you can also `set up other storage, or customize your own storage`. By using DBStorage and grom, you can use any type of db to store transaction data and state. This block can only be executed once when the program is initialized.

```go
db, err := gorm.Open("mysql", "root:root1234@/gtm?charset=utf8&parseTime=True&loc=Local")
if err != nil {
	log.Fatalf("db open failed: %v", err)
}

gtm.SetStorage(gtm.NewDBStorage(db))
```

If you use `DBStorage`, you need to create the following tables.
```sql
DROP TABLE gtm_transactions;
CREATE TABLE gtm_transactions (
	id         bigint UNSIGNED NOT NULL AUTO_INCREMENT,
	name       varchar(50) NOT NULL,
	times      int UNSIGNED NOT NULL,
	retry_at   timestamp NOT NULL,
	timeout    int UNSIGNED NOT NULL,
	result     varchar(20) NOT NULL,
	content    mediumtext,
	created_at timestamp NOT NULL,
	updated_at timestamp NOT NULL,

	PRIMARY KEY (id),
	KEY idx_retry (result, retry_at)
);

DROP TABLE gtm_partner_result;
CREATE TABLE gtm_partner_result (
	id              bigint UNSIGNED NOT NULL AUTO_INCREMENT,
	transaction_id  bigint UNSIGNED NOT NULL,
	phase           varchar(20) NOT NULL,
	offset          tinyint UNSIGNED NOT NULL,
	result          varchar(20) NOT NULL,
	cost            int UNSIGNED NOT NULL,
	created_at      timestamp NOT NULL,
	updated_at      timestamp NOT NULL,

	PRIMARY KEY (id),
	UNIQUE KEY uni_tx_id (transaction_id, phase, offset)
);
```

### Start a New Transaction
There may be three kinds of return results for each transaction, which need to be processed separately.

```go
tx := gtm.New()
tx.AddNormal(&Payer{OrderID: "100001", UserID: 20001, Amount: 99})
tx.AddUncertain(&OrderCreator{OrderID: "100001", UserID: 20001, ProductID: 31, Amount: 99})

switch result, err := tx.Execute(); result {
case gtm.Success:
	t.Logf("tx's result = success")
case gtm.Fail:
	t.Logf("tx's result = fail. err = %+v", err)
case gtm.Uncertain:
	t.Logf("tx's result = uncertain: err = %v", err)
}
```

### Retry Timeout Transactions
`RetryTimeoutTransactions` can set the number of transactions to retry each time, and finally return the retryed transactions, the results and errors of each transaction.

```go
transactions, results, errs, err := gtm.RetryTimeoutTransactions(10)
```

## Implement the Partner
You can choose to implement the three partners defined in `partner.go`.

## Customize the Storage
In addition to the built-in `DBStroage`, you can also customize your own storage engine to achieve better efficiency. For this, you need to implement the `gtm.Storage` interface.

It is recommended to use `persistent storage` for transaction data, and the state of the participants can be stored in a faster memory.

## More Documents
https://pkg.go.dev/mod/github.com/quanhengzhuang/gtm
