# GTM
GTM's full name is `Global Transaction Manager`, a framework for solving distributed transaction problems. GTM is improved based on 2PC, and easier to use than 2PC. Compared to 2PC, which requires participants to implement three functions, many participants in GTM only need to implement one function.

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

### Start a New Transaction
There may be three kinds of return results for each transaction, which need to be processed separately.

```go
tx := gtm.New()
tx.AddNormalPartners(&Payer{OrderID: "100001", UserID: 20001, Amount: 99})
tx.AddUncertainPartner(&OrderCreator{OrderID: "100001", UserID: 20001, ProductID: 31, Amount: 99})

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
