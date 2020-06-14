package gtm_test

import (
	"encoding/gob"
	"testing"

	"github.com/quanhengzhuang/gtm"
)

func init() {
	storage := NewLevelStorage()
	gob.RegisterName("*gtm.Payer", &Payer{})
	gob.RegisterName("*gtm.OrderCreator", &OrderCreator{})
	// storage.Register(&Payer{})
	// storage.Register(&OrderCreator{})

	gtm.SetStorage(storage)
}

func TestNew(t *testing.T) {
	tx := gtm.New()
	tx.AddNormalPartners(&Payer{"100000000001", 10001, 99})
	tx.AddUncertainPartner(&OrderCreator{1990001, 10001, 11, 99})

	switch result, err := tx.Execute(); result {
	case gtm.Success:
		t.Logf("tx's result is success")
	case gtm.Fail:
		t.Logf("tx's result is fail. err = %+v", err)
	case gtm.Uncertain:
		t.Errorf("tx's result is uncertain: err = %v", err)
	}
}

func TestRetry(t *testing.T) {
	if transactions, results, errs, err := gtm.RetryTimeoutTransactions(10); err != nil {
		t.Errorf("retry err: %v", err)
	} else {
		for k, tx := range transactions {
			t.Logf("retry id = %v, result = %v, err = %v", tx.ID, results[k], errs[k])
		}
	}
}
