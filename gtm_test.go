package gtm

import (
	"testing"
)

func init() {
	storage := NewLevelStorage()
	storage.Register(&Payer{})
	storage.Register(&OrderCreator{})
	SetStorage(storage)
}

func TestNew(t *testing.T) {
	tx := New()
	tx.AddNormalPartners(&Payer{"100000000001", 10001, 99})
	tx.AddUncertainPartner(&OrderCreator{1990001, 10001, 11, 99})

	switch result, err := tx.Execute(); result {
	case Success:
		t.Logf("tx's result is success")
	case Fail:
		t.Logf("tx's result is fail. err = %+v", err)
	case Uncertain:
		t.Errorf("tx's result is uncertain: err = %v", err)
	}
}

func TestRetry(t *testing.T) {
	if transactions, results, errs, err := RetryTimeoutTransactions(10); err != nil {
		t.Errorf("retry err: %v", err)
	} else {
		for k, tx := range transactions {
			t.Logf("retry id = %v, result = %v, err = %v", tx.ID, results[k], errs[k])
		}
	}
}
