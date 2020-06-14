package gtm

import (
	"log"
	"testing"
)

func init() {
	storage := NewLevelStorage()
	storage.Register(&AmountChanger{})
	storage.Register(&OrderCreator{})
	SetStorage(storage)
}

func TestRetry(t *testing.T) {
	transactions, err := GetTimeoutTransactions(100)
	t.Logf("get timeout transactions: %v", len(transactions))

	if err != nil {
		t.Errorf("get timeout transactions err: %v", err)
	}

	for _, tx := range transactions {
		log.Printf("id = %v, retryTime = %v", tx.ID, tx.RetryTime)
		if _, err := tx.ExecuteContinue(); err != nil {
			t.Errorf("tx execute continue err: %v", err)
		}
	}
}

func TestNew(t *testing.T) {
	amount := AmountChanger{1990001, 10001, 99, 0, "test"}
	order := OrderCreator{1990001, 10001, 11, 99}

	// new
	gtm := New()
	gtm.AddPartners([]NormalPartner{&amount}, &order, nil)
	ok, err := gtm.Execute()
	if err != nil {
		t.Errorf("gtm Execute() err: %v", err)
	}

	t.Logf("gtm result = %v", ok)
}
