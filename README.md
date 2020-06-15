# GTM
GTM's full name is `Global Transaction Manager`, a framework for solving distributed transaction problems. GTM is improved based on 2PC, and easier to use than 2PC. Compared to 2PC, which requires participants to implement three functions, many participants in GTM only need to implement one function.

## Usage
### Install
```
go get github.com/quanhengzhuang/gtm
```

### Start A New Transaction
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
default:
	t.Errorf("unexpected result: %v", result)
}
```

### Retry Timeout Transactions
```go
count := 10
if transactions, results, errs, err := gtm.RetryTimeoutTransactions(count); err != nil {
	t.Errorf("retry err: %v", err)
} else {
	for k, tx := range transactions {
		t.Logf("retry id = %v, result = %v, err = %v", tx.ID, results[k], errs[k])
	}
}
```

## Implement The Partner
You can choose to implement the three partners defined in `partner.go`.

## Custom Storage
You should implement the gtm.Storage interface.

A storage example based on LevelDB is provided in `storage_test.go`, but this is only an example and cannot be used for production. It is recommended to use MySQL + Redis to achieve transaction storage in production.

## More Documents
https://pkg.go.dev/mod/github.com/quanhengzhuang/gtm
