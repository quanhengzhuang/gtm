# gtm
Global Transaction Manager

## Import
```go
import (
	"github.com/quanhengzhuang/gtm"
)
```

## Start a new transaction
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

## Retry timeout transactions
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